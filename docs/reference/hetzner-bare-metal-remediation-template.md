## HetznerBareMetalRemediationTemplate

In ```HetznerBareMetalRemediationTemplate``` you can define all important properties for ```HetznerBareMetalRemediations```. With this remediation, you can define a custom method for the manner of how Machine Health Checks treat the unhealthy objects - `HetznerBareMetalMachines` in this case. For more information about how to use remdiations, see [Advanced CAPH](/docs/topics/advanced-caph.md). ```HetznerBareMetalRemediations``` are reconciled by the ```HetznerBareMetalRemediationController```, which reconciles the remediatons and triggers the requested type of remediation on the relevant `HetznerBareMetalMachine`.

### Overview of HetznerBareMetalRemediationTemplate.Spec
| Key | Type | Default | Required | Description |
|-----|-----|------|---------|-------------|
| template.spec.strategy | object |  | yes | Remediation strategy to be applied |
| template.spec.strategy.type | string | Reboot  | no | Type of the remediation strategy. At the moment, only "Reboot" is supported |
| template.spec.strategy.retryLimit | int | 0 | no | Set maximum of remediation retries. Zero retries if not set. |
| template.spec.strategy.timeout | string | | yes | Timeout of one remediation try. Should be of the form "10m", or "40s" |
