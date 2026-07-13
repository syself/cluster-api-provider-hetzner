---
title: HCloudRemediationTemplate
description: RemediationStrategyTypes define remediation strategy, timeouts and retries.
metatitle: RemediationStrategyTypes Object Reference
---

## Overview of HCloudMachineTemplate.Spec

<PropField name="template.spec.strategy" type="object" required={false}>

Strategy field defines remediation strategy.

<Collapsible title="properties">

<PropField name="template.spec.strategy.retryLimit" type="integer" required={false}>
RetryLimit sets the maximum number of remediation retries. Zero retries if not set.
</PropField>

<PropField name="template.spec.strategy.timeout" type="string" required={true}>
Timeout sets the timeout between remediation retries. It should be of the form "10m", or "40s".
</PropField>

<PropField name="template.spec.strategy.types" type="string" required={false}>
Type represents the type of the remediation strategy. At the moment, only "Reboot" is supported.
</PropField>

</Collapsible>

</PropField>
