---
title: HCloudRemediationTemplate
metatitle: "Remediation Strategy Types: Specification and Categories"
sidebar: HCloudRemediationTemplate
description: Discover various remediation strategies and learn about different types to enhance your security posture. Explore how to handle security incidents effectively.
---

### Overview of HCloudMachineTemplate.Spec

| Key                                        | Type       | Default                                 | Required | Description                                                                                                                                                                                                                                                                                     |
| ------------------------------------------ | ---------- | --------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `template.spec.strategy`                 | `object`   |                                         | no       | Strategy field defines remediation strategy                                                                                                                                                                                                                                                                    |
| `template.spec.strategy.retryLimit`                       | `integer`   |                                         | no      | RetryLimit sets the maximum number of remediation retries. Zero retries if not set                                                                                                                                                                                                                            |
| `template.spec.strategy.timeout`                  | `string`   |                                         | yes      | Timeout sets the timeout between remediation retries. It should be of the form "10m", or "40s" |
| `template.spec.strategy.types`                    | `string`   |                                         | no       | Type represents the type of the remediation strategy. At the moment, only "Reboot" is supported                                                                                                                                                                                                                                                         |
