---
title: HetznerBareMetalRemediationTemplate
description: With this remediation, you can define a custom method for how Machine Health Checks treats unhealthy HetznerBareMetalMachine objects.
metatitle: HetznerBareMetalRemediationTemplate Object Reference
---

In `HetznerBareMetalRemediationTemplate` you can define all important properties for `HetznerBareMetalRemediations`. With this remediation, you can define a custom method for the manner of how Machine Health Checks treat the unhealthy `object`s - `HetznerBareMetalMachines` in this case. For more information about how to use remediations, see [Advanced CAPH](/docs/caph/02-topics/06-advanced/04-custom-templates-mhc.md). `HetznerBareMetalRemediations` are reconciled by the `HetznerBareMetalRemediationController`, which reconciles the remediations and triggers the requested type of remediation on the relevant `HetznerBareMetalMachine`.

## Overview of HetznerBareMetalRemediationTemplate.Spec

<PropField name="template.spec.strategy" type="object" required={true}>

Remediation strategy to be applied.

<Collapsible title="properties">

<PropField name="template.spec.strategy.type" type="string" defaultValue="Reboot" required={false}>
Type of the remediation strategy. At the moment, only "Reboot" is supported.
</PropField>

<PropField name="template.spec.strategy.retryLimit" type="int" defaultValue="0" required={false}>
Set maximum of remediation retries. Zero retries if not set.
</PropField>

<PropField name="template.spec.strategy.timeout" type="string" required={true}>
Timeout of one remediation try. Should be of the form "10m", or "40s".
</PropField>

<PropField name="template.spec.strategy.cooldown" type="string" defaultValue="30m" required={false}>
Minimum time between two remediations of the same CAPI Machine. If a new remediation starts within this time after the last successful one, CAPH skips the reboot, and the CAPI Machine gets deleted. Set it to "0s" to turn this check off.
</PropField>

<PropField name="template.spec.strategy.onExhaustion" type="string" required={false}>
What to do when the retries run out and the node is still unhealthy. `Reuse` deletes the CAPI Machine and frees the host to be provisioned again. If not set, CAPH behaves like `Reuse`. `Retire` sets a permanent error on the host, which deletes the CAPI Machine and keeps the host out of the pool until someone removes the `capi.syself.com/permanent-error` annotation.
</PropField>

</Collapsible>

</PropField>
