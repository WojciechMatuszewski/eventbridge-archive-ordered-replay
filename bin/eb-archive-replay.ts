#!/usr/bin/env node
import "source-map-support/register";
import * as cdk from "aws-cdk-lib";
import { EBArchiveReplayStack } from "../lib/eb-archive-replay-stack";
import { IAspect, RemovalPolicy, CfnResource } from "aws-cdk-lib";
import { IConstruct } from "constructs";

const app = new cdk.App();
const stack = new EBArchiveReplayStack(app, "InfraStack", {
  synthesizer: new cdk.DefaultStackSynthesizer({
    qualifier: "ebarchre"
  })
});

class DeletionPolicySetter implements IAspect {
  constructor(private readonly policy: RemovalPolicy) {}
  visit(node: IConstruct): void {
    if (node instanceof CfnResource) {
      node.applyRemovalPolicy(this.policy);
    }
  }
}

cdk.Aspects.of(stack).add(new DeletionPolicySetter(cdk.RemovalPolicy.DESTROY));
