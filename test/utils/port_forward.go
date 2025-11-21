package utils

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

// RequestFunc defines a function that makes an HTTP request given a base URL (host:port)
type RequestFunc func(baseURL string) (body []byte, statusCode int, err error)

// resolveServiceToPod resolves a service to one of its backing pods and the target port
func resolveServiceToPod(
	ctx context.Context, clientset *kubernetes.Clientset, namespace, serviceName string, servicePort int,
) (*corev1.Pod, int, error) {
	// Get the service
	service, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get service: %w", err)
	}

	// Check if service has selectors
	if len(service.Spec.Selector) == 0 {
		return nil, 0, fmt.Errorf("service %s does not have a selector", serviceName)
	}

	// Find the service port definition
	var targetPort *int
	var targetPortName string
	for _, port := range service.Spec.Ports {
		if port.Port == int32(servicePort) {
			// Found the matching service port
			if port.TargetPort.Type == 0 { // IntVal
				tp := int(port.TargetPort.IntVal)
				targetPort = &tp
			} else { // StrVal
				targetPortName = port.TargetPort.StrVal
			}
			break
		}
	}

	if targetPort == nil && targetPortName == "" {
		return nil, 0, fmt.Errorf("service %s does not expose port %d", serviceName, servicePort)
	}

	// Convert service selector to label selector
	set := labels.Set(service.Spec.Selector)
	listOptions := metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	}

	// List pods matching the service selector
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, 0, fmt.Errorf("no pods found for service %s", serviceName)
	}

	// Filter for running pods and select the first ready one
	var selectedPod *corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			// Check if pod is ready
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					selectedPod = &pod
					break
				}
			}
			if selectedPod != nil {
				break
			}
		}
	}

	// If no ready pod found, use first running pod
	if selectedPod == nil {
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				selectedPod = &pod
				break
			}
		}
	}

	if selectedPod == nil {
		return nil, 0, fmt.Errorf("no running pods found for service %s", serviceName)
	}

	// Resolve named port if necessary
	if targetPortName != "" {
		resolved := false
		for _, container := range selectedPod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == targetPortName {
					tp := int(port.ContainerPort)
					targetPort = &tp
					resolved = true
					break
				}
			}
			if resolved {
				break
			}
		}
		if !resolved {
			return nil, 0, fmt.Errorf("could not resolve named port %s in pod %s", targetPortName, selectedPod.Name)
		}
	}

	if targetPort == nil {
		return nil, 0, fmt.Errorf("could not determine target port for service %s", serviceName)
	}

	return selectedPod, *targetPort, nil
}

// PortForwardPod creates a port forward to a specific pod
func PortForwardPod(ctx context.Context, namespace, podName string, port int) (int, error) {
	// Get Kubernetes REST config
	config, err := rest.InClusterConfig()
	if err != nil {
		// If not in cluster, try to use kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return 0, fmt.Errorf("failed to get kubernetes config: %w", err)
		}
	}

	// Build the port forward URL - must be to a pod
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)

	serverURL, err := neturl.Parse(config.Host)
	if err != nil {
		return 0, fmt.Errorf("failed to parse host URL: %w", err)
	}
	serverURL.Path = path

	// Create a SPDY roundtripper
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return 0, fmt.Errorf("failed to create roundtripper: %w", err)
	}

	// Create a dialer
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)

	// Set up streams
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)
	out := GinkgoWriter
	errOut := GinkgoWriter

	// Use port 0 to get a random local port
	ports := []string{fmt.Sprintf("0:%d", port)}

	// Create the portforwarder
	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return 0, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Start port forwarding in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			errChan <- err
		}
	}()

	// Wait for the port forwarder to be ready or fail
	select {
	case <-readyChan:
		// Port forwarder is ready
	case err := <-errChan:
		return 0, fmt.Errorf("port forwarding failed: %w", err)
	case <-ctx.Done():
		close(stopChan)
		return 0, ctx.Err()
	}

	// Get the actual local port that was assigned
	forwardedPorts, err := forwarder.GetPorts()
	if err != nil {
		close(stopChan)
		return 0, fmt.Errorf("failed to get forwarded ports: %w", err)
	}
	if len(forwardedPorts) == 0 {
		close(stopChan)
		return 0, fmt.Errorf("no ports were forwarded")
	}
	localPort := int(forwardedPorts[0].Local)

	_, _ = fmt.Fprintf(GinkgoWriter, "port-forward established: 127.0.0.1:%d -> %s/%s:%d",
		localPort, namespace, podName, port)

	// Monitor context and stop port forwarding when canceled
	go func() {
		<-ctx.Done()
		_, _ = fmt.Fprintf(GinkgoWriter, "stopping port-forward for %s/%s\n", namespace, podName)
		close(stopChan)
	}()

	return localPort, nil
}

// PortForwardService creates a port forward to a service by resolving it to a pod
func PortForwardService(ctx context.Context, namespace, serviceName string, port int) (int, error) {
	// Get Kubernetes REST config
	config, err := rest.InClusterConfig()
	if err != nil {
		// If not in cluster, try to use kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return 0, fmt.Errorf("failed to get kubernetes config: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Resolve service to pod and target port
	pod, targetPort, err := resolveServiceToPod(ctx, clientset, namespace, serviceName, port)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve service to pod: %w", err)
	}

	_, _ = fmt.Fprintf(GinkgoWriter, "resolved service %s (port %d) to pod %s (port %d)",
		serviceName, port, pod.Name, targetPort)

	// Port forward to the resolved pod using the target port
	return PortForwardPod(ctx, namespace, pod.Name, targetPort)
}

// MakeServiceRequest establishes a port-forward to the service, makes an HTTP request, and cleans up.
// The requestFunc receives the base URL (e.g., "http://localhost:12345") and performs the actual request.
// Non-2xx HTTP status codes are returned successfully (not treated as errors), allowing callers
// to verify specific status codes like 404.
func MakeServiceRequest(
	namespace, serviceName string,
	servicePort int,
	requestFunc RequestFunc,
) (body []byte, statusCode int, err error) {
	// Create fresh port-forward for this request
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localPort, pfErr := PortForwardService(ctx, namespace, serviceName, servicePort)
	if pfErr != nil {
		return nil, 0, fmt.Errorf("port-forward failed: %w", pfErr)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", localPort)
	return requestFunc(baseURL)
}

// MakeServiceGet is a convenience wrapper for GET requests to a Kubernetes service.
func MakeServiceGet(namespace, serviceName string, servicePort int, endpoint string) ([]byte, int, error) {
	return MakeServiceRequest(
		namespace, serviceName, servicePort,
		func(baseURL string) ([]byte, int, error) {
			return GetRequestWithStatus(baseURL + endpoint)
		},
	)
}

// MakeServicePost is a convenience wrapper for POST requests to a Kubernetes service.
func MakeServicePost(
	namespace, serviceName string,
	servicePort int,
	endpoint string,
	payload interface{},
) ([]byte, int, error) {
	return MakeServiceRequest(
		namespace, serviceName, servicePort,
		func(baseURL string) ([]byte, int, error) {
			return PostRequestWithStatus(baseURL+endpoint, payload)
		},
	)
}
