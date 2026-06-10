import * as cdk from 'aws-cdk-lib';
import * as path from 'path';
import * as fs from 'fs';
import { fileURLToPath } from 'url';
import { ResourceName, AwsService, Plane } from '@cmd-saas/libs-infra-resource';
import { ObservePlaneDomain } from './domains.js';
import { SnsStack } from './stacks/sns-stack.js';
import { AlarmsStack } from './stacks/alarms-stack.js';
import { LogAlarmsStack } from './stacks/log-alarms-stack.js';
import { loadAlarmEnvConfig, loadResourceInventory, emptyInventory } from './alarm-config.js';

// Resolve paths relative to this launcher file
const alarmsRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

const app = new cdk.App();

// --- Config ---------------------------------------------------------------
const configPathContext = app.node.tryGetContext('config') as string | undefined;
if (!configPathContext) {
  throw new Error(
    'Config path is required. Provide with -c config=<path/to/config.json>\n' +
    'Example: npx cdk synth -c config=./config/main.json',
  );
}

const configPath = path.resolve(alarmsRoot, configPathContext);
const envConfig = loadAlarmEnvConfig(configPath);

// Derive environment name from tags
const environment = envConfig.tags['environment'];
if (!environment) {
  throw new Error(
    'Config file must contain an "environment" tag. ' +
    `Check ${configPath}`,
  );
}

// --- Destroy check --------------------------------------------------------
const destroyContext = app.node.tryGetContext('destroy');
const isDestroy =
  destroyContext === true ||
  (typeof destroyContext === 'string' && destroyContext.toLowerCase() === 'true');

// --- CDK env (auto-resolve from credentials if not set) -------------------
const cdkEnv = (process.env.CDK_DEFAULT_ACCOUNT && process.env.CDK_DEFAULT_REGION)
  ? { account: process.env.CDK_DEFAULT_ACCOUNT, region: process.env.CDK_DEFAULT_REGION }
  : undefined;

// --- SNS Stack (one per environment, shared by all alarm stacks) ----------
const snsStackName = new ResourceName({
  plane: Plane.OP,
  environment,
  domain: ObservePlaneDomain.METRICS,
  purposeDescriptor: 'alarms-sns',
  awsService: AwsService.CLOUDFORMATION,
}).toString();

const snsStack = new SnsStack(app, snsStackName, {
  env: cdkEnv,
  environment,
  alarmEmail: envConfig.alarmEmail,
  description: `Observe Plane — Alarm SNS topic (${environment})`,
});

// --- Create alarm stacks from scope inventory files -----------------------
const multiDir = path.resolve(alarmsRoot, 'generated/multi');
const manifestPath = path.join(multiDir, 'manifest.json');

if (isDestroy && !fs.existsSync(manifestPath)) {
  // Destroy mode without inventory — CDK needs at least the SNS stack to destroy.
  // No alarm stacks to create.
} else if (!fs.existsSync(manifestPath)) {
  throw new Error(
    `Scope manifest not found: ${manifestPath}\n` +
    `Run: task discover config=<config.json> first.`,
  );
} else {
  const manifest: string[] = JSON.parse(fs.readFileSync(manifestPath, 'utf-8')) as string[];

  for (const scope of manifest) {
    const inventoryPath = path.join(multiDir, `${scope}.json`);
    const inventory = fs.existsSync(inventoryPath)
      ? loadResourceInventory(inventoryPath)
      : emptyInventory();

    const stackName = new ResourceName({
      plane: Plane.OP,
      environment,
      domain: ObservePlaneDomain.METRICS,
      purposeDescriptor: `alarms-${scope}`,
      awsService: AwsService.CLOUDFORMATION,
    }).toString();

    const alarmsStack = new AlarmsStack(app, stackName, {
      env: cdkEnv,
      environment,
      topicArn: snsStack.topicArn,
      inventory,
      description: `Observe Plane — CloudWatch alarms (${environment} / ${scope})`,
    });

    alarmsStack.addDependency(snsStack);
  }
}

// --- Log-based alarms stack (config-driven, not from discovery) ------------
const logAlarmsStackName = new ResourceName({
  plane: Plane.OP,
  environment,
  domain: ObservePlaneDomain.METRICS,
  purposeDescriptor: 'alarms-logs',
  awsService: AwsService.CLOUDFORMATION,
}).toString();

const logAlarmsStack = new LogAlarmsStack(app, logAlarmsStackName, {
  env: cdkEnv,
  environment,
  topicArn: snsStack.topicArn,
  description: `Observe Plane — Log-based alarms (${environment})`,
});
logAlarmsStack.addDependency(snsStack);

app.synth();