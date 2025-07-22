local must_env = std.native('must_env');
{
  region: must_env('AWS_REGION'),
  cluster: 'default',
  service: 'test',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  timeout: '10m0s',
  ignore: {
    tags: [
      'ecspresso:ignore',
    ],
  },
  plugins: [
    {
      name: 'tfstate',
      config: {
        path: 'terraform.tfstate',
      },
    },
  ],
}
