---
title: HCloudRemediationTemplate
metatitle: RemediationStrategyTypes Object Reference
sidebar: HCloudRemediationTemplate
description: RemediationStrategyTypes define remediation strategy, timeouts and retries.
---

## Overview of HCloudMachineTemplate.Spec

| Key                                 | Type      | Default | Required | Description                                                                                     |
| ----------------------------------- | --------- | ------- | -------- | ----------------------------------------------------------------------------------------------- |
| `template.spec.strategy`            | `object`  |         | no       | Strategy field defines remediation strategy                                                     |
| `template.spec.strategy.retryLimit` | `integer` |         | no       | RetryLimit sets the maximum number of remediation retries. Zero retries if not set              |
| `template.spec.strategy.timeout`    | `string`  |         | yes      | Timeout sets the timeout between remediation retries. It should be of the form "10m", or "40s"  |
| `template.spec.strategy.types`      | `string`  |         | no       | Type represents the type of the remediation strategy. At the moment, only "Reboot" is supported |
