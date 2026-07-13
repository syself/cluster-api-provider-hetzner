---
title: HetznerBareMetalRemediationTemplate
description: With this remediation, you can define a custom method for how Machine Health Checks treats unhealthy HetznerBareMetalMachine objects.
metatitle: HetznerBareMetalRemediationTemplate Object Reference
---

In `HetznerBareMetalRemediationTemplate` you can define all important properties for `HetznerBareMetalRemediations`. With this remediation, you can define a custom method for the manner of how Machine Health Checks treat the unhealthy `object`s - `HetznerBareMetalMachines` in this case. For more information about how to use remediations, see [Advanced CAPH](/docs/caph/topics/advanced/custom-templates-mhc). `HetznerBareMetalRemediations` are reconciled by the `HetznerBareMetalRemediationController`, which reconciles the remediatons and triggers the requested type of remediation on the relevant `HetznerBareMetalMachine`.

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

</Collapsible>

</PropField>
