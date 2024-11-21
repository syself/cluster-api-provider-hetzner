---
title: Rate limits
metatitle: Rate Limits for HCloudMachine Reconciliation Controller
sidebar: Rate limits
description: Information about Hetzner Cloud rate limits, and what to expect when you are affected.
---

Hetzner Cloud and Hetzner Robot both implement rate limits. As a brute-force method, we implemented some logic that prevents the controller from reconciling a specific object for some defined time period if a rate limit was hit during reconcilement of that object. We set the condition on true, that a rate limit was hit. Of course, this only affects one object so that another `HCloudMachine` still reconciles normally, even though one hits the rate limit. There is a chance that it will also hit the rate limit (which is defined per function so that it does not necessarily need to happen). In that case, the controller also stops reconciling this object for some time.
