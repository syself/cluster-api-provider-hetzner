package checkconditions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Arguments struct {
	Verbose           bool
	Sleep             time.Duration
	WhileRegex        *regexp.Regexp
	ProgrammStartTime time.Time
	Name              string
	// NamespacePatterns is the raw input from -n. Each entry may be an exact
	// namespace name or a glob pattern (*, ?, [...]).
	NamespacePatterns []string
	// Namespaces is the resolved list after matching patterns against the
	// cluster's namespaces. Populated by RunAndGetCounter.
	Namespaces []string
	// ExcludeNamespacePatterns is the raw input from --exclude-namespace.
	// Each entry may be an exact namespace name or a glob pattern. Matched
	// per-object at filter time; non-matching entries are not an error.
	ExcludeNamespacePatterns []string
	RetryCount               int16
	RetryForEver             bool
	Timeout                  time.Duration
	// WarnDeletionTimestampOlderThan warns about resources whose deletionTimestamp
	// is older than this duration. Set to 0 to disable.
	WarnDeletionTimestampOlderThan time.Duration
	forbiddenResourcesPrinted      bool
}

// matchAnyPattern reports whether name matches any of the given glob patterns.
func matchAnyPattern(name string, patterns []string) bool {
	for _, p := range patterns {
		if ok, err := path.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}

// validatePatterns returns an error if any pattern has invalid glob syntax.
func validatePatterns(patterns []string) error {
	for _, p := range patterns {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("invalid namespace pattern %q: %w", p, err)
		}
	}
	return nil
}

// namespaceSet returns a lookup set of resolved namespaces. Empty when no
// namespace filter is in effect.
func (a *Arguments) namespaceSet() map[string]struct{} {
	if len(a.Namespaces) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(a.Namespaces))
	for _, ns := range a.Namespaces {
		m[ns] = struct{}{}
	}
	return m
}

// namespaceFilterActive reports whether the user requested a namespace filter
// (regardless of whether the patterns have been resolved yet).
func (a *Arguments) namespaceFilterActive() bool {
	return len(a.NamespacePatterns) > 0
}

func patternHasGlob(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// resolveNamespacePatterns expands the user-supplied patterns into concrete
// namespace names. Patterns without glob characters are kept as-is. Patterns
// with glob characters are matched against the namespaces that exist in the
// cluster. Returns an error if no namespace matches.
func resolveNamespacePatterns(ctx context.Context, clientset *kubernetes.Clientset, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	hasGlob := false
	for _, p := range patterns {
		if patternHasGlob(p) {
			hasGlob = true
			break
		}
	}

	seen := map[string]struct{}{}
	var resolved []string

	if !hasGlob {
		// Trust the user-supplied names. Verifying existence here would
		// require cluster-scoped get/list permission on namespaces, which
		// namespace-scoped service accounts don't have.
		for _, p := range patterns {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			resolved = append(resolved, p)
		}
		return resolved, nil
	}

	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, fmt.Errorf("cannot expand glob patterns %v: listing namespaces is not allowed for this user; use exact namespace names instead", patterns)
		}
		return nil, fmt.Errorf("error listing namespaces: %w", err)
	}
	for _, p := range patterns {
		if !patternHasGlob(p) {
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				resolved = append(resolved, p)
			}
			continue
		}
		for i := range nsList.Items {
			name := nsList.Items[i].Name
			ok, err := path.Match(p, name)
			if err != nil {
				return nil, fmt.Errorf("invalid namespace pattern %q: %w", p, err)
			}
			if !ok {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			resolved = append(resolved, name)
		}
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("no namespace matches patterns %v", patterns)
	}
	slices.Sort(resolved)
	return resolved, nil
}

var resourcesToSkip = []schema.GroupResource{
	{Group: "", Resource: "bindings"},
	{Group: "", Resource: "componentstatuses"},
	{Group: "", Resource: "configmaps"},        // no status subresource
	{Group: "", Resource: "endpoints"},         // Deprecated in 1.33+
	{Group: "", Resource: "events"},            // no status subresource
	{Group: "", Resource: "limitranges"},       // no status subresource
	{Group: "", Resource: "persistentvolumes"}, // PersistentVolumeStatus has phase only, no conditions
	{Group: "", Resource: "podtemplates"},      // no status subresource
	{Group: "", Resource: "resourcequotas"},    // ResourceQuotaStatus has hard/used, no conditions
	{Group: "", Resource: "secrets"},           // no status subresource
	{Group: "", Resource: "serviceaccounts"},   // no status subresource
	{Group: "apps", Resource: "controllerrevisions"}, // no status subresource
	{Group: "authorization.k8s.io", Resource: "localsubjectaccessreviews"},
	{Group: "authorization.k8s.io", Resource: "selfsubjectaccessreviews"},
	{Group: "authorization.k8s.io", Resource: "selfsubjectreviews"},
	{Group: "authorization.k8s.io", Resource: "selfsubjectrulesreviews"},
	{Group: "authorization.k8s.io", Resource: "subjectaccessreviews"},
	{Group: "authentication.k8s.io", Resource: "selfsubjectreviews"},
	{Group: "authentication.k8s.io", Resource: "tokenreviews"},
	{Group: "batch", Resource: "cronjobs"}, // CronJobStatus has no conditions field
}

type Counter struct {
	CheckedResources     int32
	CheckedConditions    int32
	CheckedResourceTypes int32
	StartTime            time.Time
	WhileRegexDidMatch   bool
	Lines                []string
	ForbiddenResources   []string
}

func (c *Counter) add(o handleResourceTypeOutput) {
	c.CheckedResources += o.checkedResources
	c.CheckedConditions += o.checkedConditions
	c.CheckedResourceTypes += o.checkedResourceTypes
	c.Lines = append(c.Lines, o.lines...)
	if o.forbiddenResource != "" {
		c.ForbiddenResources = append(c.ForbiddenResources, o.forbiddenResource)
	}
	if o.whileRegexDidMatch {
		c.WhileRegexDidMatch = true
	}
}

// RunAllOnce returns true if an unhealthy condition was found.
func RunAllOnce(ctx context.Context, args *Arguments) (bool, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return false, fmt.Errorf("error creating client config: %w", err)
	}

	// 80 concurrent requests were served in roughly 200ms
	// This means 400 requests in one second (to local kind cluster)
	// But why reduce this? I don't want people with better hardware
	// to wait for getting results from an api-server running at localhost
	config.QPS = 1000
	config.Burst = 1000
	return RunCheckAllConditions(ctx, config, args)
}

func RunForever(ctx context.Context, args *Arguments) error {
	for {
		_, err := RunAllOnce(ctx, args)
		if err != nil {
			return err
		}
		time.Sleep(args.Sleep)
		fmt.Printf("\n%s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"))
	}
}

func RunWhileRegex(ctx context.Context, arguments *Arguments) error {
	for {
		again, err := runWhileInner(ctx, arguments)
		if err != nil {
			return err
		}
		if !again {
			return nil
		}
	}
}

// return true if the while-regex matched
func runWhileInner(ctx context.Context, arguments *Arguments) (bool, error) {
	unhealthy, err := RunAllOnce(ctx, arguments)
	if err != nil {
		return false, err
	}
	if !unhealthy {
		fmt.Printf("Regex %q did not match. Stopping\n", arguments.WhileRegex.String())
		return false, nil
	}
	pre := fmt.Sprintf("Regex %q did match. ", arguments.WhileRegex.String())

	d := time.Since(arguments.ProgrammStartTime)
	durationStr := d.Round(time.Second).String()
	if arguments.Timeout > 0 {
		untilTimeout := arguments.Timeout - d
		durationStr += ", timeout in " + untilTimeout.Round(time.Second).String()
	}
	fmt.Printf("%sWaiting %s, then checking again. %s (%s).\n\n",
		pre,
		arguments.Sleep.String(),
		time.Now().Format("2006-01-02 15:04:05 -0700 MST"),
		durationStr)
	time.Sleep(time.Duration(arguments.Sleep))
	return true, nil
}

// If arguments.WhileRegex, then return true if there was a matching unhealthy condition.
// Otherwise return true if there was at least one unhealthy condition.
func RunCheckAllConditions(ctx context.Context, config *restclient.Config, args *Arguments) (bool, error) {
	// Get the list of all API resources available
	var err error
	var counter Counter
	var i int16
	for {
		if args.Timeout > 0 {
			d := time.Since(args.ProgrammStartTime)
			if d > args.Timeout {
				d := d.Round(time.Second)
				return false, fmt.Errorf("timeout reached after %s", d.String())
			}
		}
		counter, err = RunAndGetCounter(ctx, config, args)
		if err == nil {
			// Successful connection, from now on retry forever.
			args.RetryForEver = true
			break
		}
		var netError net.Error
		if !errors.As(err, &netError) {
			return false, err
		}
		if args.RetryForEver {
			if i%10 == 0 {
				fmt.Printf("a network error occured. Will retry forever: %v\n",
					err)
			}
		} else {
			if i > args.RetryCount {
				return false, fmt.Errorf("network error: %w", err)
			}
			fmt.Printf("a network error occured. Will retry %d times: %v\n",
				args.RetryCount-i, err)
		}
		time.Sleep(1 * time.Second)
		i++
		continue
	}

	for _, line := range counter.Lines {
		fmt.Println(line)
	}
	if len(counter.ForbiddenResources) > 0 && !args.forbiddenResourcesPrinted {
		seen := map[string]struct{}{}
		uniq := make([]string, 0, len(counter.ForbiddenResources))
		for _, r := range counter.ForbiddenResources {
			if _, ok := seen[r]; ok {
				continue
			}
			seen[r] = struct{}{}
			uniq = append(uniq, r)
		}
		slices.Sort(uniq)
		fmt.Printf("Skipped %d forbidden resource types: %s\n", len(uniq), strings.Join(uniq, ", "))
		args.forbiddenResourcesPrinted = true
	}
	name := args.Name
	if name != "" {
		name = " (" + name + ")"
	}
	scope := " in all namespaces"
	switch len(args.Namespaces) {
	case 0:
		// no filter
	case 1:
		scope = fmt.Sprintf(" in namespace %s", args.Namespaces[0])
	default:
		scope = fmt.Sprintf(" in namespaces %s", strings.Join(args.Namespaces, ","))
	}
	fmt.Printf("Checked %d conditions of %d resources of %d types%s. Duration: %s%s\n",
		counter.CheckedConditions, counter.CheckedResources, counter.CheckedResourceTypes, scope, time.Since(counter.StartTime).Round(time.Millisecond), name)

	if args.WhileRegex == nil {
		// "all" command
		if len(counter.Lines) > 0 {
			return true, nil
		}
		return false, nil
	}

	return counter.WhileRegexDidMatch, nil
}

func RunAndGetCounter(ctx context.Context, config *restclient.Config, args *Arguments) (Counter, error) {
	counter := Counter{StartTime: time.Now()}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return counter, fmt.Errorf("error creating clientset: %w", err)
	}

	if err := validatePatterns(args.ExcludeNamespacePatterns); err != nil {
		return counter, err
	}
	if args.namespaceFilterActive() {
		// Resolve once per run; subsequent retries reuse the resolved list.
		if len(args.Namespaces) == 0 {
			resolved, err := resolveNamespacePatterns(ctx, clientset, args.NamespacePatterns)
			if err != nil {
				return counter, err
			}
			args.Namespaces = resolved
		}
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return counter, fmt.Errorf("error creating dynamic client: %w", err)
	}

	discoveryClient := clientset.Discovery()

	serverResources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			fmt.Printf("WARNING: The Kubernetes server has an orphaned API service. Server reports: %s\n", err.Error())
			fmt.Printf("WARNING: To fix this, kubectl delete apiservice <service-name>\n")
		} else {
			return counter, fmt.Errorf("error getting server preferred resources: %w", err)
		}
	}

	jobs := make(chan handleResourceTypeInput)
	results := make(chan handleResourceTypeOutput)
	var wg sync.WaitGroup

	// Concurrency needed?
	// Without: 320ms
	// With 10 or more workers: 190ms

	createWorkers(ctx, &wg, jobs, results)

	var wgCounter sync.WaitGroup
	wgCounter.Add(1)
	go func() {
		for result := range results {
			counter.add(result)
		}
		wgCounter.Done()
	}()

	createJobs(serverResources, jobs, args, dynClient)

	close(jobs)
	wg.Wait()
	close(results)
	wgCounter.Wait()
	slices.Sort(counter.Lines)
	return counter, nil
}

func createJobs(serverResources []*metav1.APIResourceList, jobs chan handleResourceTypeInput, args *Arguments, dynClient *dynamic.DynamicClient) {
	for _, resourceList := range serverResources {
		groupVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse group version: %v\n", err)
			continue
		}
		for i := range resourceList.APIResources {
			namespaced := resourceList.APIResources[i].Namespaced
			if args.namespaceFilterActive() && !namespaced {
				continue
			}
			jobs <- handleResourceTypeInput{
				args:      args,
				dynClient: dynClient,
				gvr: schema.GroupVersionResource{
					Group:    groupVersion.Group,
					Version:  groupVersion.Version,
					Resource: resourceList.APIResources[i].Name,
				},
				namespaced: namespaced,
			}
		}
	}
}

func createWorkers(ctx context.Context, wg *sync.WaitGroup, jobs chan handleResourceTypeInput, results chan handleResourceTypeOutput) {
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int32) {
			defer wg.Done()
			for input := range jobs {
				input.workerID = workerID
				results <- handleResourceType(ctx, input)
			}
		}(int32(i))
	}
}

func containsSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}

// printResources returns true if the conditions should get checked again N seconds later.
func printResources(args *Arguments, list *unstructured.UnstructuredList, gvr schema.GroupVersionResource,
	counter *handleResourceTypeOutput, workerID int32,
) (lines []string, again bool) {
	// When the user supplied multiple include namespaces (or globs), we list
	// resources cluster-wide and drop the ones not in the resolved set. A
	// single include is already filtered server-side. Excludes always apply
	// here for namespaced lists (cluster-scoped resources have no namespace).
	var nsInclude map[string]struct{}
	if len(args.Namespaces) > 1 {
		nsInclude = args.namespaceSet()
	}
	for _, obj := range list.Items {
		ns := obj.GetNamespace()
		if nsInclude != nil {
			if _, ok := nsInclude[ns]; !ok {
				continue
			}
		}
		if ns != "" && matchAnyPattern(ns, args.ExcludeNamespacePatterns) {
			continue
		}
		counter.checkedResources++
		if args.WarnDeletionTimestampOlderThan > 0 {
			if dt := obj.GetDeletionTimestamp(); dt != nil && !dt.IsZero() {
				age := time.Since(dt.Time)
				if age > args.WarnDeletionTimestampOlderThan {
					line := fmt.Sprintf("  %s %s %s DeletionTimestamp set for %s",
						obj.GetNamespace(), gvr.Resource, obj.GetName(), age.Round(time.Second))
					if args.WhileRegex == nil || args.WhileRegex.MatchString(line) {
						if args.WhileRegex != nil {
							again = true
						}
						lines = append(lines, line)
					}
				}
			}
		}
		var conditions []interface{}
		var err error
		if gvr.Resource == "hetznerbaremetalhosts" {
			// For some reasons this resource stores the conditions differently
			conditions, _, err = unstructured.NestedSlice(obj.Object, "spec", "status", "conditions")
		} else {
			conditions, _, err = unstructured.NestedSlice(obj.Object, "status", "conditions")
		}
		if err != nil {
			if strings.Contains(err.Error(), "<nil> is of the type <nil>") {
				// If we read the manifest before the controller created conditions, then
				// this can happen.
				continue
			}
			lines = append(lines, fmt.Sprintf("err of unstructured.NestedSlice(%+v): %s",
				obj.Object, err.Error()))
		}
		subLines, a := printConditions(args, conditions, counter, gvr, obj)
		if a {
			again = true
		}
		lines = append(lines, subLines...)
	}
	if args.Verbose {
		fmt.Printf("    checked %s %s %s workerID=%d\n", gvr.Resource, gvr.Group, gvr.Version, workerID)
	}
	return lines, again
}

type conditionRow struct {
	conditionType               string
	conditionStatus             string
	conditionReason             string
	conditionMessage            string
	conditionLastTransitionTime time.Time
}

var readyString = "Ready"

// printConditions returns true if the conditions should be checked again N seconds later.
func printConditions(args *Arguments, conditions []interface{}, counter *handleResourceTypeOutput,
	gvr schema.GroupVersionResource, obj unstructured.Unstructured,
) (lines []string, again bool) {
	var rows []conditionRow
	for _, condition := range conditions {
		rows = handleCondition(condition, counter, gvr, rows)
	}
	// remove general ready condition, if it is already contained in a particular condition
	// https://pkg.go.dev/sigs.k8s.io/cluster-api/util/conditions#SetSummary
	var ready *conditionRow
	for i := range rows {
		if rows[i].conditionType == readyString {
			ready = &rows[i]
			break
		}
	}
	skipReadyCondition := false
	if ready != nil {
		for _, r := range rows {
			if r.conditionType == readyString {
				continue
			}
			if r.conditionMessage == ready.conditionMessage &&
				r.conditionReason == ready.conditionReason &&
				r.conditionStatus == ready.conditionStatus {
				skipReadyCondition = true
				break
			}
		}
	}
	// Merge rows that share the same (status, reason, message) into one output line.
	// This avoids duplicate lines when multiple condition types carry identical information
	// (e.g. Failed and FailureTarget both reporting BackoffLimitExceeded).
	type mergeKey struct{ status, reason, message string }
	type mergedEntry struct {
		row   conditionRow
		types []string
	}
	var order []mergeKey
	byKey := map[mergeKey]*mergedEntry{}
	for _, r := range rows {
		if skipReadyCondition && r.conditionType == readyString {
			continue
		}
		k := mergeKey{r.conditionStatus, r.conditionReason, r.conditionMessage}
		if e, ok := byKey[k]; ok {
			e.types = append(e.types, r.conditionType)
			if !r.conditionLastTransitionTime.IsZero() &&
				(e.row.conditionLastTransitionTime.IsZero() ||
					r.conditionLastTransitionTime.Before(e.row.conditionLastTransitionTime)) {
				e.row.conditionLastTransitionTime = r.conditionLastTransitionTime
			}
		} else {
			byKey[k] = &mergedEntry{row: r, types: []string{r.conditionType}}
			order = append(order, k)
		}
	}

	for _, k := range order {
		e := byKey[k]
		slices.Sort(e.types)
		r := e.row
		r.conditionType = strings.Join(e.types, "/")

		duration := ""
		if !r.conditionLastTransitionTime.IsZero() {
			d := time.Since(r.conditionLastTransitionTime)
			duration = fmt.Sprint(d.Round(time.Second))
		}

		outLine := fmt.Sprintf("  %s %s %s Condition %s=%s %s %q (%s)", obj.GetNamespace(), gvr.Resource, obj.GetName(), r.conditionType, r.conditionStatus,
			r.conditionReason, r.conditionMessage, duration)

		addLine := true
		if args.WhileRegex != nil {
			addLine = false
			// Check each individual type for backward compatibility with --while regexes
			// that match on a specific condition type name (e.g. "Failed=True").
			for _, t := range e.types {
				singleLine := fmt.Sprintf("  %s %s %s Condition %s=%s %s %q (%s)",
					obj.GetNamespace(), gvr.Resource, obj.GetName(),
					t, r.conditionStatus, r.conditionReason, r.conditionMessage, duration)
				if args.WhileRegex.MatchString(singleLine) {
					again = true
					addLine = true
					break
				}
			}
			// Also check the merged line (handles regexes that match the combined type string).
			if !addLine && args.WhileRegex.MatchString(outLine) {
				again = true
				addLine = true
			}
		}

		if addLine {
			lines = append(lines, outLine)
		}
	}
	return lines, again
}

func handleCondition(condition interface{}, counter *handleResourceTypeOutput, gvr schema.GroupVersionResource, rows []conditionRow) []conditionRow {
	conditionMap, ok := condition.(map[string]interface{})
	if !ok {
		fmt.Println("Invalid condition format")
		return rows
	}
	counter.checkedConditions++

	conditionType, _ := conditionMap["type"].(string)
	conditionStatus, _ := conditionMap["status"].(string)
	if conditionToSkip(conditionType) {
		return rows
	}
	switch conditionStatus {
	case "True":
		if conditionTypeHasPositiveMeaning(gvr.Resource, conditionType) {
			return rows
		}
	case "False":
		if conditionTypeHasNegativeMeaning(gvr.Resource, conditionType) {
			return rows
		}
	}
	conditionReason, _ := conditionMap["reason"].(string)
	conditionMessage, _ := conditionMap["message"].(string)
	conditionLine := fmt.Sprintf("%s %s=%s %s %q", gvr.Resource, conditionType, conditionStatus, conditionReason, conditionMessage)
	for _, r := range conditionLinesToIgnoreRegexs {
		if r.MatchString(conditionLine) {
			return rows
		}
	}
	if conditionDone(conditionType, conditionStatus, conditionReason) {
		return rows
	}
	s, _ := conditionMap["lastTransitionTime"].(string)
	conditionLastTransitionTime := time.Time{}
	if s != "" {
		conditionLastTransitionTime, _ = time.Parse(time.RFC3339, s)
	}
	rows = append(rows, conditionRow{
		conditionType, conditionStatus,
		conditionReason, conditionMessage, conditionLastTransitionTime,
	})
	return rows
}

func conditionToSkip(ct string) bool {
	// Skip conditions which can be True or False, and both values are fine.
	toSkip := []string{
		"DisruptionAllowed",
		"LoadBalancerAttachedToNetwork",
		"NetworkAttached",
		"PodReadyToStartContainers", // completed pods have "False".
		"RefVersionsUpToDate",       // happens during api version transition.
	}
	return slices.Contains(toSkip, ct)
}

var conditionTypesOfResourceWithPositiveMeaning = map[string][]string{
	"extensionconfigs": { // runtime.cluster.x-k8s.io
		"Discovered",
	},
	"hetznerclusters": {
		"ControlPlaneEndpointSet",
	},
	"hetznerbaremetalmachines": {
		"AssociateBMHCondition",
	},
	"horizontalpodautoscalers": {
		"AbleToScale",
		"ScalingActive",
	},
	"hetznerbaremetalhosts": {
		"RootDeviceHintsValidated",
		"NodeBootIDRetrieved",
	},
	"clusters": {
		"ConsistentSystemID",    // postgresql.cnpg.io/v1
		"ContinuousArchiving",   // postgresql.cnpg.io/v1
		"RemoteConnectionProbe", // capi
	},
	"clusteraddons": {
		"ClusterAddonConfigValidated",
		"ClusterAddonHelmChartUntarred",
	},
	"engineimages": { // Longhorn
		"ready",
	},
	"gitrepositories": { // source.toolkit.fluxcd.io/v1
		"ArtifactInStorage",
	},
	"nodes": {
		"Schedulable",         // Longhorn
		"MountPropagation",    // Longhorn
		"RequiredPackages",    // Longhorn
		"KernelModulesLoaded", // Longhorn
		"EtcdIsVoter",
	},
	"machines": {
		"NodeKubeadmLabelsAndTaintsSet",
	},
	"autopilotclusters": {
		"ClusterRunning",
	},
	"applicationsets": {
		"ParametersGenerated",
	},
}

var conditionTypesOfResourceWithNegativeMeaning = map[string][]string{
	"nodes": {
		"KernelDeadlock",
		"ReadonlyFilesystem",
		"FrequentUnregisterNetDevice",
		"NTPProblem",
		"CperHardwareErrorFatal",
		"DisksFailure",
		"KubeletNeedsRestart",
		"XfsShutdown",
	},
	"horizontalpodautoscalers": {
		"ScalingLimited",
	},
	"clusters": {
		"RollingOut", // capi
		"ScalingDown",
		"ScalingUp",
		"Remediating",
	},
	"kubeadmcontrolplanes": {
		"RollingOut",
		"ScalingDown",
		"ScalingUp",
		"Remediating",
	},
	"machinedeployments": {
		"RollingOut",
		"ScalingDown",
		"ScalingUp",
		"Remediating",
	},
	"machinesets": {
		"ScalingDown",
		"ScalingUp",
		"Remediating",
	},
	"machines": {
		"Updating",
	},
	"applicationsets": {
		"ErrorOccurred",
	},
}

// To create new IngoreRegex take the line you see and remove the namespace, the resource name and the time from that line.
// Example:
// from: longhorn-system backuptargets myname Condition Unavailable=True Unavailable "backup target URL is empty" (5m21s)
// to: `backuptargets Unavailable=True Unavailable "backup target URL is empty"`
var conditionLinesToIgnoreRegexs = []*regexp.Regexp{
	// Cluster API
	regexp.MustCompile("machinesets MachinesReady=False Deleted @.*"),
	regexp.MustCompile("machinesets (MachinesReady|Ready)=False DrainingFailed @."),
	regexp.MustCompile("machinesets Ready=False Deleted @.*"),
	regexp.MustCompile(`machinesets (MachinesReady|Ready)=False NodeNotFound.*`),
	regexp.MustCompile(`machinedeployments (MachineSetReady|Ready)=False Deleted.*`),

	// Longhorn
	regexp.MustCompile(`backuptargets Unavailable=True Unavailable "backup target URL is empty"`),
	regexp.MustCompile(`engines InstanceCreation=True`),
	regexp.MustCompile(`engines FilesystemReadOnly=False`),
	regexp.MustCompile(`replicas InstanceCreation=True`),
	regexp.MustCompile(`replicas FilesystemReadOnly=False`),
	regexp.MustCompile(`replicas WaitForBackingImage=False`),
	regexp.MustCompile(`volumes WaitForBackingImage=False`),
	regexp.MustCompile(`volumes TooManySnapshots=False`),
	regexp.MustCompile(`volumes Scheduled=True`),
	regexp.MustCompile(`volumes Restore=False`),

	// perconaxtradbclusters
	regexp.MustCompile(`perconaxtradbclusters tls=enabled`),
}

func conditionTypeHasPositiveMeaning(resource string, ct string) bool {
	types := conditionTypesOfResourceWithPositiveMeaning[resource]
	if slices.Contains(types, ct) {
		return true
	}

	for _, suffix := range []string{
		"Applied",
		"Approved",
		"Available",
		"Built",
		"Complete",
		"Created",
		"Downloaded",
		"Established",
		"Healthy",
		"Initialized",
		"Installed",
		"LoadBalancerAttached",
		"NamesAccepted",
		"Passed",
		"PodScheduled",
		"Progressing",
		"ProviderUpgraded",
		"Provisioned",
		"Reachable",
		"Ready",
		"Reconciled",
		"RemediationAllowed",
		"Resized",
		"Succeeded",
		"Synced",
		"UpToDate",
		"Valid",
		"SuccessCriteriaMet",
		"RunningDesiredVersion", // elasticsearches
		"ready",                 // perconaservermongodbs
		"sharding",              // perconaservermongodbs
		"Conformant",            // customresourcedefinitions KubernetesAPIApprovalPolicyConformant
		"NoWarnings",            // rabbitmqclusters
		"ReconcileSuccess",      // rabbitmqclusters
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	for _, prefix := range []string{
		"Created",
	} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

func conditionDone(conditionType string, conditionStatus string, conditionReason string) bool {
	// machinesets demo-1-md-0-q9qzp-6gsw9 Condition MachinesReady=False Deleted @ Machine/demo-1-md-0-q9qzp-6gsw9-vkxrp ""
	// The reason contains "@ ...". We need to split that
	if conditionType == "MachinesReady" {
		parts := strings.Split(conditionReason, "@")
		conditionType = strings.TrimSpace(parts[0])
	}

	if slices.Contains([]string{"Ready", "ContainersReady", "InfrastructureReady", "MachinesReady"}, conditionType) &&
		slices.Contains([]string{"PodCompleted", "InstanceTerminated", "Deleted"}, conditionReason) &&
		conditionStatus == "False" {
		return true
	}
	return false
}

func conditionTypeHasNegativeMeaning(resource string, ct string) bool {
	types := conditionTypesOfResourceWithNegativeMeaning[resource]
	if slices.Contains(types, ct) {
		return true
	}

	for _, suffix := range []string{
		"Unavailable", "Pressure", "Dangling", "Unhealthy", "Paused", "Deleting", "Failed",
	} {
		if strings.HasSuffix(ct, suffix) {
			return true
		}
	}
	if strings.HasPrefix(ct, "Frequent") && strings.HasSuffix(ct, "Restart") {
		return true
	}

	return false
}

type handleResourceTypeInput struct {
	args       *Arguments
	dynClient  *dynamic.DynamicClient
	gvr        schema.GroupVersionResource
	workerID   int32
	namespaced bool
}

type handleResourceTypeOutput struct {
	checkedResourceTypes int32
	checkedResources     int32
	checkedConditions    int32
	whileRegexDidMatch   bool
	lines                []string
	// forbiddenResource is the resource name when listing was rejected with a
	// 403 Forbidden. Aggregated by the caller into a single summary line.
	forbiddenResource string
}

func handleResourceType(ctx context.Context, input handleResourceTypeInput) handleResourceTypeOutput {
	var output handleResourceTypeOutput

	args := input.args
	name := input.gvr.Resource
	dynClient := input.dynClient
	gvr := input.gvr
	// Skip subresources like pod/logs, pod/status
	if containsSlash(name) {
		return output
	}
	if slices.Contains(resourcesToSkip, gvr.GroupResource()) {
		return output
	}

	if args.namespaceFilterActive() && !input.namespaced {
		return output
	}

	namespaceable := dynClient.Resource(gvr)
	var resourceInterface dynamic.ResourceInterface
	if input.namespaced {
		// One namespace and no excludes: filter on the server. Otherwise list
		// cluster-wide and apply include/exclude filters in printResources.
		canServerSideFilter := len(args.Namespaces) == 1 && len(args.ExcludeNamespacePatterns) == 0
		if canServerSideFilter {
			resourceInterface = namespaceable.Namespace(args.Namespaces[0])
		} else if len(args.Namespaces) == 1 && matchAnyPattern(args.Namespaces[0], args.ExcludeNamespacePatterns) {
			// Single included namespace is itself excluded — nothing to do.
			return output
		} else {
			resourceInterface = namespaceable.Namespace(metav1.NamespaceAll)
		}
	} else {
		resourceInterface = namespaceable
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			output.forbiddenResource = name
			return output
		}
		fmt.Printf("..Error listing %s: %v. group %q version %q resource %q\n", name, err,
			gvr.Group, gvr.Version, gvr.Resource)
		return output
	}

	output.checkedResourceTypes++
	lines, again := printResources(args, list, gvr, &output, input.workerID)
	output.whileRegexDidMatch = again
	output.lines = lines
	return output
}
