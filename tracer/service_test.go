package tracer

import (
	"testing"

	commonPb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcePb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		name     string
		resource *resourcePb.Resource
		want     string
	}{
		{
			name: "single_valid_service_name",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{
					{
						Key: "service.name",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "auth-service",
							},
						},
					},
				},
			},
			want: "auth-service",
		},
		{
			name: "non_string_service_name_value",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{
					{
						Key: "service.name",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_IntValue{
								IntValue: 42,
							},
						},
					},
				},
			},
			want: "", // Expect empty string for non-string values
		},
		{
			name: "multiple_attributes_with_service_name",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{
					{
						Key: "environment",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "production",
							},
						},
					},
					{
						Key: "service.name",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "order-service",
							},
						},
					},
				},
			},
			want: "order-service",
		},
		{
			name: "no_attributes",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{},
			},
			want: "",
		},
		{
			name: "attributes_without_service_name",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{
					{
						Key: "environment",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "staging",
							},
						},
					},
				},
			},
			want: "",
		},
		{
			name: "multiple_service_name_entries",
			resource: &resourcePb.Resource{
				Attributes: []*commonPb.KeyValue{
					{
						Key: "service.name",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "first-service",
							},
						},
					},
					{
						Key: "service.name",
						Value: &commonPb.AnyValue{
							Value: &commonPb.AnyValue_StringValue{
								StringValue: "second-service",
							},
						},
					},
				},
			},
			want: "first-service", // Returns first occurrence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getServiceName(tt.resource)
			if got != tt.want {
				t.Errorf("getServiceName() = %q, want %q", got, tt.want)
			}
		})
	}
}
