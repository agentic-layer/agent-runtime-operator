/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	runtimev1alpha1 "github.com/agentic-layer/agent-runtime-operator/api/v1alpha1"
)

const (
	testImageV1     = "test:v1"
	testImageNode   = "node:20"
	testImagePython = "python:3.11"
)

var _ = Describe("ToolServer Webhook", func() {
	var (
		obj       *runtimev1alpha1.ToolServer
		oldObj    *runtimev1alpha1.ToolServer
		validator ToolServerCustomValidator
		defaulter ToolServerCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		obj = &runtimev1alpha1.ToolServer{}
		oldObj = &runtimev1alpha1.ToolServer{}
		validator = ToolServerCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = ToolServerCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		ctx = context.Background()
	})

	AfterEach(func() {
		// No teardown needed
	})

	Context("When creating ToolServer under Defaulting Webhook", func() {
		It("Should set default protocol when not specified", func() {
			By("having no protocol set initially")
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Protocol).To(BeEmpty())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default protocol is set")
			Expect(obj.Spec.Protocol).To(Equal(mcpProtocol))
		})

		It("Should not override existing protocol", func() {
			By("setting a custom protocol")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing protocol is preserved")
			Expect(obj.Spec.Protocol).To(Equal(mcpProtocol))
		})

		It("Should set default port for http transport when not specified", func() {
			By("having http transport without port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Port).To(Equal(int32(0)))

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default port is set")
			Expect(obj.Spec.Port).To(Equal(int32(8080)))
		})

		It("Should set default port for sse transport when not specified", func() {
			By("having sse transport without port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = sseTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Port).To(Equal(int32(0)))

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default port is set")
			Expect(obj.Spec.Port).To(Equal(int32(8080)))
		})

		It("Should not set default port for stdio transport", func() {
			By("having stdio transport")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageV1

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that port remains unset")
			Expect(obj.Spec.Port).To(Equal(int32(0)))
		})

		It("Should not override existing port", func() {
			By("setting a custom port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			obj.Spec.Port = 9090

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing port is preserved")
			Expect(obj.Spec.Port).To(Equal(int32(9090)))
		})

		It("Should set default replicas for http transport when not specified", func() {
			By("having http transport without replicas")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Replicas).To(BeNil())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default replicas are set")
			Expect(obj.Spec.Replicas).NotTo(BeNil())
			Expect(*obj.Spec.Replicas).To(Equal(int32(1)))
		})

		It("Should set default replicas for sse transport when not specified", func() {
			By("having sse transport without replicas")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = sseTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Replicas).To(BeNil())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default replicas are set")
			Expect(obj.Spec.Replicas).NotTo(BeNil())
			Expect(*obj.Spec.Replicas).To(Equal(int32(1)))
		})

		It("Should not set default replicas for stdio transport", func() {
			By("having stdio transport")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageV1

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that replicas remain unset")
			Expect(obj.Spec.Replicas).To(BeNil())
		})

		It("Should not override existing replicas", func() {
			By("setting custom replicas")
			replicas := int32(3)
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			obj.Spec.Replicas = &replicas

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing replicas are preserved")
			Expect(*obj.Spec.Replicas).To(Equal(int32(3)))
		})

		It("Should set default path for http transport when not specified", func() {
			By("having http transport without path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Path).To(BeEmpty())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default path is set")
			Expect(obj.Spec.Path).To(Equal("/mcp"))
		})

		It("Should set default path for sse transport when not specified", func() {
			By("having sse transport without path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = sseTransport
			obj.Spec.Image = testImageV1
			Expect(obj.Spec.Path).To(BeEmpty())

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that default path is set")
			Expect(obj.Spec.Path).To(Equal("/sse"))
		})

		It("Should not set default path for stdio transport", func() {
			By("having stdio transport")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageV1

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that path remains unset")
			Expect(obj.Spec.Path).To(BeEmpty())
		})

		It("Should not override existing path", func() {
			By("setting a custom path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImageV1
			obj.Spec.Path = "/api/mcp"

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			By("verifying that existing path is preserved")
			Expect(obj.Spec.Path).To(Equal("/api/mcp"))
		})
	})

	Context("When creating or updating ToolServer under Validating Webhook", func() {
		It("Should deny creation if image is missing", func() {
			By("creating a ToolServer without image")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image must be specified"))
		})

		It("Should admit creation if image is specified", func() {
			By("creating a valid stdio ToolServer")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageNode

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation if stdio transport has port set", func() {
			By("creating a stdio ToolServer with port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageNode
			obj.Spec.Port = 8080

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port must not be specified for stdio transport"))
		})

		It("Should deny creation if stdio transport has replicas set", func() {
			By("creating a stdio ToolServer with replicas")
			replicas := int32(2)
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageNode
			obj.Spec.Replicas = &replicas

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas must not be specified for stdio transport"))
		})

		It("Should deny creation if stdio transport has path set", func() {
			By("creating a stdio ToolServer with path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = stdioTransport
			obj.Spec.Image = testImageNode
			obj.Spec.Path = "/mcp"

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path must not be specified for stdio transport"))
		})

		It("Should deny creation if http transport has no port", func() {
			By("creating an http ToolServer without port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port is required for http transport"))
		})

		It("Should admit creation if http transport has port", func() {
			By("creating a valid http ToolServer")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation if sse transport has no port", func() {
			By("creating an sse ToolServer without port")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = sseTransport
			obj.Spec.Image = testImagePython

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("port is required for sse transport"))
		})

		It("Should admit creation if sse transport has port", func() {
			By("creating a valid sse ToolServer")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = sseTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate updates correctly", func() {
			By("setting up old and new valid objects")
			oldObj.Spec.Protocol = mcpProtocol
			oldObj.Spec.TransportType = httpTransport
			oldObj.Spec.Image = testImagePython
			oldObj.Spec.Port = 8080

			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = "python:3.12"
			obj.Spec.Port = 9090

			By("validating the update")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny update if new object is invalid", func() {
			By("setting up old valid object and new invalid object")
			oldObj.Spec.Protocol = mcpProtocol
			oldObj.Spec.TransportType = httpTransport
			oldObj.Spec.Image = testImagePython
			oldObj.Spec.Port = 8080

			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = ""
			obj.Spec.Port = 8080

			By("validating the update")
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image must be specified"))
		})
	})

	Context("When validating advanced constraints", func() {
		It("Should deny creation if http transport has negative replicas", func() {
			By("creating an http ToolServer with negative replicas")
			replicas := int32(-1)
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080
			obj.Spec.Replicas = &replicas

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas cannot be negative"))
		})

		It("Should allow zero replicas for http transport", func() {
			By("creating an http ToolServer with zero replicas")
			replicas := int32(0)
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080
			obj.Spec.Replicas = &replicas

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny creation if path is invalid for http transport", func() {
			By("creating an http ToolServer with invalid path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080
			obj.Spec.Path = "invalid-path"

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to fail")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path must start with '/'"))
		})

		It("Should allow valid path for http transport", func() {
			By("creating an http ToolServer with valid path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080
			obj.Spec.Path = "/api/mcp/v1"

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should allow root path for http transport", func() {
			By("creating an http ToolServer with root path")
			obj.Spec.Protocol = mcpProtocol
			obj.Spec.TransportType = httpTransport
			obj.Spec.Image = testImagePython
			obj.Spec.Port = 8080
			obj.Spec.Path = "/"

			By("validating the creation")
			_, err := validator.ValidateCreate(ctx, obj)

			By("expecting validation to succeed")
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
