local tfstate = std.native('tfstate');
{
  deploymentConfiguration: {
    maximumPercent: 200,
    minimumHealthyPercent: 100,
    deploymentCircuitBreaker: {
      enable: true,
      rollback: true,
    },
  },
  desiredCount: 1,
  enableECSManagedTags: false,
  launchType: 'EC2',
  loadBalancers: [],
  placementConstraints: [],
  placementStrategy: [],
  schedulingStrategy: 'REPLICA',
  serviceRegistries: [],
  networkConfiguration: {
    awsvpcConfiguration: {
      subnets: [
        tfstate('aws_subnet.private-a.id'),
      ],
      securityGroups: [
        subnet.id for subnet in std.objectValues(tfstate('data.aws_security_group.default'))
      ],
      assignPublicIp: 'ENABLED',
    },
  },
  tags: [
    {
      key: 'Name',
      value: 'test',
    },
    {
      key: 'ecspresso:ignore',
      value: 'true',
    },
  ],
}
