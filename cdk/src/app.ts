#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { createAlphaStacks } from './env/alpha';
import { createPipeline } from './env/pipeline';

const app = new cdk.App();

if (process.env.DEV_ACCOUNT) {
  createAlphaStacks(app, process.env.DEV_ACCOUNT);
} else {
  createPipeline(app);
}