local env = std.native('env');
local must_env = std.native('must_env');
local isCodeDeploy = env('DEPLOYMENT_CONTROLLER', 'ECS') == 'CODE_DEPLOY';
{
  availabilityZoneRebalancing: 'ENABLED',
  capacityProviderStrategy: [
    {
      base: 1,
      capacityProvider: 'FARGATE',
      weight: 1,
    },
    {
      base: 0,
      capacityProvider: 'FARGATE_SPOT',
      weight: 2,
    },
  ],
  deploymentConfiguration: {
    deploymentCircuitBreaker: if isCodeDeploy then null else {
      enable: true,
      rollback: true,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: env('DEPLOYMENT_CONTROLLER', 'ECS'),
  },
  desiredCount: std.parseInt(env('DESIRED_COUNT', '1')),
  enableECSManagedTags: false,
  enableExecuteCommand: std.parseJson(env('ENABLE_EXECUTE_COMMAND', 'true')),
  healthCheckGracePeriodSeconds: 0,
  loadBalancers: [
    {
      containerName: 'nginx',
      containerPort: 80,
      targetGroupArn: 'arn:aws:elasticloadbalancing:ap-northeast-1:{{must_env `AWS_ACCOUNT_ID`}}:targetgroup/alpha/6a301850702273d9',
    },
  ],
  networkConfiguration: {
    awsvpcConfiguration: {
      assignPublicIp: 'ENABLED',
      securityGroups: [
        'sg-0a69199a34e15147a',
        'sg-0c09a8157ba2cfa22',
      ],
      subnets: [
        'subnet-0623adfcb3093f18f',
        'subnet-0376f113bbbc25742',
        'subnet-04b750544ddd71274',
      ],
    },
  },
  placementConstraints: [],
  placementStrategy: [],
  platformVersion: '1.4.0',
  schedulingStrategy: 'REPLICA',
  serviceRegistries: [],
  serviceConnectConfiguration: {
    enabled: true,
    namespace: 'ecspresso-test',
  },
  propagateTags: 'SERVICE',
  tags: [
    {
      key: 'deployed_at',
      value: '{{ env `NOW` `` }}',
    },
    {
      key: 'cluster',
      value: 'ecspresso-test',
    },
    {
      key: 'cost-category',
      value: 'ecspresso-test',
    },
  ],
  volumeConfigurations: if isCodeDeploy then null else [
    {
      managedEBSVolume: {
        filesystemType: 'ext4',
        roleArn: 'arn:aws:iam::%s:role/ecsInfrastructureRole' % must_env('AWS_ACCOUNT_ID'),
        sizeInGiB: 10,
        tagSpecifications: [
          {
            propagateTags: 'SERVICE',
            resourceType: 'volume',
          },
          {
            propagateTags: 'TASK_DEFINITION',
            resourceType: 'volume',
          },
        ],
        volumeType: 'gp3',
      },
      name: 'ebs',
    },
  ],
  /*
  vpcLatticeConfigurations: [
    {
      portName: 'nginx-http',
      roleArn: 'arn:aws:iam::%s:role/ecsInfrastructureRole' % must_env('AWS_ACCOUNT_ID'),
      targetGroupArn: 'arn:aws:vpc-lattice:ap-northeast-1:%s:targetgroup/tg-009147df264a0bacb' % must_env('AWS_ACCOUNT_ID'),
    },
  ],
  */
}
