// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"remote-provider/internal/provider/services"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Ensure RemoteHostProvider satisfies various provider interfaces.
var _ provider.Provider = &RemoteHostProvider{}
var _ provider.ProviderWithFunctions = &RemoteHostProvider{}
var _ provider.ProviderWithEphemeralResources = &RemoteHostProvider{}

// RemoteHostProvider defines the provider implementation.
type RemoteHostProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// RemoteHostProviderModel describes the provider data model.
type RemoteHostProviderModel struct{}

func (p *RemoteHostProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "remote_host"
	resp.Version = p.version
}

func (p *RemoteHostProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *RemoteHostProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data RemoteHostProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	sshService := &services.SSHService{}

	resp.DataSourceData = sshService
	resp.ResourceData = sshService
}

func (p *RemoteHostProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRemoteFileResource,
	}
}

func (p *RemoteHostProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewExampleEphemeralResource,
	}
}

func (p *RemoteHostProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *RemoteHostProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewExampleFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RemoteHostProvider{
			version: version,
		}
	}
}
