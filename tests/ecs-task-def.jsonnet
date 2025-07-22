local env = std.native('env');
local must_env = std.native('must_env');
local tfstate = std.native('tfstate');
{
  containerDefinitions: [
    {
      cpu: 1024,
      essential: true,
      image: tfstate('aws_ecr_repository.all["app"].repository_url') + ':' + must_env('TAG'),
      memory: 1024,
      name: 'app',
      environment: [
        {
          name: 'JSON',
          value: env('JSON', ''),
        },
      ],
      portMappings: [
        {
          containerPort: 80,
          hostPort: 80,
          protocol: 'tcp',
        },
      ],
    },
  ],
  family: 'app',
  requiresCompatibilities: [
    'EC2',
  ],
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
