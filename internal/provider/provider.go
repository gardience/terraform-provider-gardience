package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &gardienceProvider{}

type gardienceProvider struct{ version string }

type providerModel struct {
	URL        types.String `tfsdk:"url"`
	APIKey     types.String `tfsdk:"api_key"`
	CACertFile types.String `tfsdk:"ca_cert_file"`
	Insecure   types.Bool   `tfsdk:"insecure"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &gardienceProvider{version: version} }
}

func (p *gardienceProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gardience"
	resp.Version = p.version
}

func (p *gardienceProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [Gardience](https://gardience.com), the supported Windows and Linux console on open-source Salt: " +
			"enroll the machines Terraform creates so they are patched, hardened and audited from first boot, and manage " +
			"node groups, saved states and event rules as code. Uses a Gardience **configure** API key (admins create one " +
			"in Settings > API keys). Every change is recorded in Gardience's tamper-evident audit log under the key's owner.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{Optional: true,
				MarkdownDescription: "Address of the Gardience server, for example `https://gardience.example.com`. Defaults to the `GARDIENCE_URL` environment variable."},
			"api_key": schema.StringAttribute{Optional: true, Sensitive: true,
				MarkdownDescription: "A Gardience configure key (`gdn_...`). Defaults to the `GARDIENCE_API_KEY` environment variable."},
			"ca_cert_file": schema.StringAttribute{Optional: true,
				MarkdownDescription: "PEM file with the certificate authority that signed the server's certificate, for a private CA. Defaults to `GARDIENCE_CA_CERT_FILE`."},
			"insecure": schema.BoolAttribute{Optional: true,
				MarkdownDescription: "Skip certificate checks. Only for a lab with a self-signed certificate."},
		},
	}
}

func (p *gardienceProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pick := func(v types.String, env string) string {
		if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			return v.ValueString()
		}
		return os.Getenv(env)
	}
	u, k, ca := pick(m.URL, "GARDIENCE_URL"), pick(m.APIKey, "GARDIENCE_API_KEY"), pick(m.CACertFile, "GARDIENCE_CA_CERT_FILE")
	if u == "" {
		resp.Diagnostics.AddAttributeError(path.Root("url"), "Missing Gardience URL", "Set url in the provider block or GARDIENCE_URL.")
	}
	if k == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing Gardience API key", "Set api_key in the provider block or GARDIENCE_API_KEY.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := NewClient(u, k, ca, m.Insecure.ValueBool(), p.version)
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure the Gardience provider", err.Error())
		return
	}
	resp.ResourceData, resp.DataSourceData = c, c
}

func (p *gardienceProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewEnrollmentResource, NewNodeGroupResource, NewStateResource, NewEventRuleResource, NewMachineResource, NewRunbookResource, NewScheduleResource, NewChangeWindowResource}
}

func (p *gardienceProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewBootstrapScriptDataSource, NewMachineDataSource}
}

// clientFrom unpacks the configured client for resources and data sources.
func clientFrom(data any) *Client {
	if c, ok := data.(*Client); ok {
		return c
	}
	return nil
}
