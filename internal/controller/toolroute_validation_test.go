package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

var _ = Describe("ToolRoute CEL validation", func() {
	ctx := context.Background()
	ns := "default"

	newRoute := func(name string, u runtimev1alpha1.ToolRouteUpstream) *runtimev1alpha1.ToolRoute {
		return &runtimev1alpha1.ToolRoute{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: runtimev1alpha1.ToolRouteSpec{
				ToolGatewayRef: corev1.ObjectReference{Name: "tg"},
				Upstream:       u,
			},
		}
	}

	AfterEach(func() {
		// Clean up all ToolRoutes in the test namespace
		routes := &runtimev1alpha1.ToolRouteList{}
		_ = k8sClient.List(ctx, routes, &client.ListOptions{Namespace: ns})
		for i := range routes.Items {
			_ = k8sClient.Delete(ctx, &routes.Items[i])
		}
	})

	It("accepts upstream with only toolServerRef", func() {
		r := newRoute("valid-tsref", runtimev1alpha1.ToolRouteUpstream{
			ToolServerRef: &corev1.ObjectReference{Name: "ts"},
		})
		Expect(k8sClient.Create(ctx, r)).To(Succeed())
	})

	It("accepts upstream with only external.url", func() {
		r := newRoute("valid-ext", runtimev1alpha1.ToolRouteUpstream{
			External: &runtimev1alpha1.ExternalUpstream{Url: "https://example.com/mcp"},
		})
		Expect(k8sClient.Create(ctx, r)).To(Succeed())
	})

	It("rejects upstream with both toolServerRef and external", func() {
		r := newRoute("invalid-both", runtimev1alpha1.ToolRouteUpstream{
			ToolServerRef: &corev1.ObjectReference{Name: "ts"},
			External:      &runtimev1alpha1.ExternalUpstream{Url: "https://example.com/mcp"},
		})
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one of toolServerRef or external"))
	})

	It("rejects upstream with neither toolServerRef nor external", func() {
		r := newRoute("invalid-empty", runtimev1alpha1.ToolRouteUpstream{})
		err := k8sClient.Create(ctx, r)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("exactly one of toolServerRef or external"))
	})
})
