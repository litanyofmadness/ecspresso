package ecspresso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/samber/lo"
	"github.com/schollz/progressbar/v3"
)

type waitUntil string

const (
	waitUntilStable   waitUntil = "stable"
	waitUntilDeployed waitUntil = "deployed"
)

type waitFunc func(ctx context.Context, sv *Service) error

type confirmFunc func(ctx context.Context) error

func (confirm confirmFunc) wrap(wait waitFunc) waitFunc {
	if confirm == nil {
		return wait
	}
	return func(ctx context.Context, sv *Service) error {
		if err := wait(ctx, sv); err != nil {
			return err
		}
		return confirm(ctx)
	}
}

func (d *App) WaitFunc(sv *Service, confirm confirmFunc, until waitUntil) (waitFunc, error) {
	defaultFunc := confirm.wrap(d.WaitServiceStable)
	if sv == nil || sv.DeploymentController == nil {
		return defaultFunc, nil
	}
	if dc := sv.DeploymentController; dc != nil {
		switch dc.Type {
		case types.DeploymentControllerTypeCodeDeploy:
			return d.WaitForCodeDeploy, nil
		case types.DeploymentControllerTypeEcs:
			switch until {
			case waitUntilDeployed:
				return confirm.wrap(d.WaitServiceDeployCompleted), nil
			case waitUntilStable, "":
				return defaultFunc, nil
			default:
				return nil, fmt.Errorf("unsupported waitUntil: %s", until)
			}
		default:
			return nil, fmt.Errorf("unsupported deployment controller type: %s", dc.Type)
		}
	}
	return defaultFunc, nil
}

func (d *App) confirmPrimaryTD(tdArn string) confirmFunc {
	return func(ctx context.Context) error {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return err
		}
		if dp, ok := sv.PrimaryDeployment(); ok {
			current := aws.ToString(dp.TaskDefinition)
			d.LogDebug("checking primary deployment %s %s == %s", *dp.Id, current, tdArn)
			if arnToName(current) != arnToName(tdArn) {
				return fmt.Errorf("task definition %s is not deployed yet. PRIMARY deployment is %s", tdArn, current)
			}
			d.LogDebug("task definition %s is deployed", tdArn)
			return nil
		}
		return fmt.Errorf("no primary deployment found")
	}
}

type WaitOption struct {
	WaitUntil string `aliases:"until" help:"Choose whether to wait for service stable or the deployment finishes. (stable|deployed)" default:"stable" enum:"stable,deployed"`
}

func (d *App) Wait(ctx context.Context, opt WaitOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	until := waitUntil(opt.WaitUntil)
	d.LogInfo("Waiting for the service %s", until)

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}
	d.LogJSON(sv.DeploymentController)
	doWait, err := d.WaitFunc(sv, nil, until)
	if err != nil {
		return err
	}
	if err := doWait(ctx, sv); err != nil {
		if errors.As(err, &errNotFound) && sv.isCodeDeploy() {
			d.LogInfo("%s", err)
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	d.LogInfo("Service is %s now. Completed!", until)
	return nil
}

func (d *App) WaitServiceStable(ctx context.Context, sv *Service) error {
	d.LogInfo("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	st := &showState{lastEventAt: time.Now()}
	go func() {
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick.C:
				if err := d.showServiceStatus(waitCtx, st); err != nil {
					d.LogWarn("%s", err.Error())
					continue
				}
			}
		}
	}()

	waiter := ecs.NewServicesStableWaiter(d.ecs, func(o *ecs.ServicesStableWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
	if err := waiter.Wait(ctx, d.DescribeServicesInput(), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait for service stable: %w", err)
	}
	cancel() // stop the showServiceStatus

	<-time.After(delayForServiceChanged)
	// show the service status once more (correct all logs)
	if err := d.showServiceStatus(ctx, st); err != nil {
		d.LogWarn("%s", err.Error())
	}
	return nil
}

func (d *App) WaitServiceDeployCompleted(ctx context.Context, sv *Service) error {
	d.LogInfo("Waiting for service deployed...(it will take a few minutes)")
	time.Sleep(10 * time.Second) // wait for new deployment created

	listResp, err := d.ecs.ListServiceDeployments(ctx, &ecs.ListServiceDeploymentsInput{
		Cluster: &d.Cluster,
		Service: &d.Service,
	})
	if err != nil {
		return fmt.Errorf("failed to list service deployments: %w", err)
	}
	if len(listResp.ServiceDeployments) == 0 {
		return errors.New("no deployments found for the service")
	}
	// find the latest deployment
	deployment := lo.MaxBy(listResp.ServiceDeployments, func(item types.ServiceDeploymentBrief, max types.ServiceDeploymentBrief) bool {
		return item.CreatedAt.After(*max.CreatedAt)
	})
	deploymentArn := deployment.ServiceDeploymentArn
	d.LogInfo("Waiting for service deployment %s to complete...", arnToName(*deploymentArn))

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	st := &showState{lastEventAt: time.Now()}
	for range tick.C {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := d.showServiceStatus(ctx, st); err != nil {
			d.LogWarn("%s", err.Error())
			continue
		}

		resp, err := d.ecs.DescribeServiceDeployments(ctx, &ecs.DescribeServiceDeploymentsInput{
			ServiceDeploymentArns: []string{*deploymentArn},
		})
		if err != nil {
			return fmt.Errorf("failed to describe service deployments: %w", err)
		}
		if len(resp.ServiceDeployments) == 1 {
			status := resp.ServiceDeployments[0].Status
			switch status {
			case types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentStatusRollbackSuccessful:
				d.LogInfo("Service deployment completed %s", status)
				return nil
			case types.ServiceDeploymentStatusStopped, types.ServiceDeploymentStatusRollbackFailed, types.ServiceDeploymentStatusStopRequested:
				return fmt.Errorf("Service deployment failed %s", status)
			default:
				d.LogDebug("Deployment %s, waiting...", status)
			}
		}
	}
	return nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *Service) error {
	d.LogDebug("wait for CodeDeploy")
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return err
	}
	out, err := d.codedeploy.ListDeployments(
		ctx,
		&codedeploy.ListDeploymentsInput{
			ApplicationName:     dp.ApplicationName,
			DeploymentGroupName: dp.DeploymentGroupName,
			IncludeOnlyStatuses: []cdTypes.DeploymentStatus{
				cdTypes.DeploymentStatusCreated,
				cdTypes.DeploymentStatusQueued,
				cdTypes.DeploymentStatusInProgress,
				cdTypes.DeploymentStatusReady,
			},
		},
	)
	if err != nil {
		return err
	}
	if len(out.Deployments) == 0 {
		return ErrNotFound("No deployments found in progress on CodeDeploy")
	}

	dpID := out.Deployments[0]
	d.LogInfo("Waiting for a deployment successful ID: " + dpID)
	go d.codeDeployProgressBar(ctx, dpID)

	waiter := codedeploy.NewDeploymentSuccessfulWaiter(d.codedeploy, func(o *codedeploy.DeploymentSuccessfulWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
	return waiter.Wait(
		ctx,
		&codedeploy.GetDeploymentInput{DeploymentId: &dpID},
		d.Timeout(),
	)
}

type showState struct {
	lastEventAt     time.Time
	deploymentsHash []byte
}

func (d *App) showServiceStatus(ctx context.Context, st *showState) error {
	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return fmt.Errorf("failed to describe services: %w", err)
	}
	if len(out.Services) == 0 {
		return ErrNotFound(fmt.Sprintf("service %s is not found", d.Service))
	}
	sv := out.Services[0]

	// show events
	sort.SliceStable(sv.Events, func(i, j int) bool {
		return sv.Events[i].CreatedAt.Before(*sv.Events[j].CreatedAt)
	})
	for _, event := range sv.Events {
		if (*event.CreatedAt).After(st.lastEventAt) {
			WriteOutput(serviceEvent(event))
			st.lastEventAt = *event.CreatedAt
		}
	}

	// show deployments
	h := sha256.New()
	lines := make([]string, 0, len(sv.Deployments))
	for _, dep := range sv.Deployments {
		line := formatDeployment(dep)
		lines = append(lines, line)
		h.Write([]byte(line))
	}
	hash := h.Sum(nil)
	// if the deployments are not changed, do not show the deployments.
	if !bytes.Equal(st.deploymentsHash, hash) {
		for _, line := range lines {
			d.LogInfo(line)
		}
	}
	st.deploymentsHash = hash
	return nil
}

func (d *App) codeDeployProgressBar(ctx context.Context, dpID string) error {
	opts := []progressbar.Option{
		progressbar.OptionSetDescription("Traffic shifted"),
		progressbar.OptionSetWidth(20),
	}
	if logFormat == logFormatJSON {
		// disable progress bar in JSON format
		opts = append(opts, progressbar.OptionSetWriter(io.Discard))
	} else {
		opts = append(opts, progressbar.OptionSetWriter(os.Stdout))
		defer func() {
			// append new line after progress bar
			os.Stdout.Write([]byte("\n"))
		}()
	}
	bar := progressbar.NewOptions(100, opts...)
	t := time.NewTicker(10 * time.Second)
	lcEvents := map[string]cdTypes.LifecycleEventStatus{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		out, err := d.codedeploy.GetDeploymentTarget(ctx, &codedeploy.GetDeploymentTargetInput{
			DeploymentId: &dpID,
			TargetId:     aws.String(d.Cluster + ":" + d.Service),
		})
		if err != nil {
			d.LogWarn("%s", err.Error())
			continue
		}
		dep := out.DeploymentTarget
		d.LogDebug("status: %s, %s", dep.EcsTarget.Status, *dep.EcsTarget.LastUpdatedAt)
		if dep.EcsTarget.Status != "InProgress" {
			break
		}
		for _, ev := range dep.EcsTarget.LifecycleEvents {
			name := *ev.LifecycleEventName
			if lcEvents[name] != ev.Status {
				if ev.Status != cdTypes.LifecycleEventStatusPending {
					d.LogInfo("%s: %s", name, ev.Status)
				}
				lcEvents[name] = ev.Status
			}
		}
		for _, element := range dep.EcsTarget.TaskSetsInfo {
			d.LogDebug("taskset: %s, %s, %f", element.TaskSetLabel, *element.Status, element.TrafficWeight)
			if *element.Status == "ACTIVE" {
				bar.Set(int(element.TrafficWeight))
			}
		}
	}
	bar.Finish()
	return nil
}

func (d *App) WaitTaskSetStable(ctx context.Context, sv *Service) error {
	var prev types.StabilityStatus
	for {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return err
		}
		switch n := len(sv.TaskSets); n {
		case 0:
			d.LogInfo("Waiting task sets available")
		default:
			ts := sv.TaskSets[0]
			if aws.ToString(ts.Status) == "PRIMARY" {
				if prev != ts.StabilityStatus {
					d.LogInfo("Waiting a task set PRIMARY stable: %s", ts.StabilityStatus)
					if n > 1 {
						d.LogInfo("Waiting a PRIMARY taskset available only")
					}
				}
				if ts.StabilityStatus == types.StabilityStatusSteadyState && n == 1 {
					d.LogInfo("Service is stable now. Completed!")
					return nil
				}
				prev = ts.StabilityStatus
			}
		}
		time.Sleep(10 * time.Second)
	}
}
