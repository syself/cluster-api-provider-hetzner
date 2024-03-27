# Rate limits

## What are rate limits?

Rate limits are restrictions that can be set by a service provider to limit the number of requests that an user can make within a specific amount of time. They are commonly used in APIs and other services. These restrictions on limiting requests are put in place to ensure that the underlying infrastructure of a server remains intact during a surge of traffic. It also helps manage load by effective distribution of server resources while maintaining the quality of service. Rate limitings also prevents abuse of the service and ensure a fair usage. It can be applied on various granularities like per-user, per-applications, etc.

## Rate limits in CAPH

Hetzner Cloud and Hetzner Robot both implement rate limits. As a brute-force method, we implemented some logic that prevents the controller from reconciling a specific object for some defined time period if a rate limit was hit during reconcilement of that object. We set the condition on true, that a rate limit was hit. Of course, this only affects one object so that another `HCloudMachine` still reconciles normally, even though one hits the rate limit. There is a chance that it will also hit the rate limit (which is defined per function so that it does not necessarily need to happen). In that case, the controller also stops reconciling this object for some time.
