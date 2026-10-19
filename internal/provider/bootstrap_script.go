package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type bootstrapScriptDataSource struct{ c *Client }

type bootstrapModel struct {
	EnrollmentID types.String `tfsdk:"enrollment_id"`
	OS           types.String `tfsdk:"os"`
	Node         types.String `tfsdk:"node"`
	Groups       types.List   `tfsdk:"groups"`
	Content      types.String `tfsdk:"content"`
}

func NewBootstrapScriptDataSource() datasource.DataSource { return &bootstrapScriptDataSource{} }

func (d *bootstrapScriptDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bootstrap_script"
}

func (d *bootstrapScriptDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A first-boot script for one machine: its name in Gardience and its own groups on top of the enrollment's. " +
			"Use `content` as the instance's `user_data`. It contains the enrollment token, so it is marked sensitive.",
		Attributes: map[string]schema.Attribute{
			"enrollment_id": schema.StringAttribute{Required: true, MarkdownDescription: "`gardience_enrollment.<name>.id`"},
			"os": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("linux", "windows")},
				MarkdownDescription: "`linux` (POSIX sh) or `windows` (PowerShell)."},
			"node":    schema.StringAttribute{Optional: true, MarkdownDescription: "The machine's name in Gardience. Default: its host name."},
			"groups":  schema.ListAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: "Extra node groups for this machine."},
			"content": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "The script."},
		},
	}
}

func (d *bootstrapScriptDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData)
}

func (d *bootstrapScriptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m bootstrapModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groups := []string{}
	m.Groups.ElementsAs(ctx, &groups, false)
	q := url.Values{"os": {m.OS.ValueString()}, "node": {m.Node.ValueString()}, "groups": {strings.Join(groups, ",")}}
	var s string
	if err := d.c.Do(ctx, "GET", fmt.Sprintf("/enrollments/%s/script?%s", url.PathEscape(m.EnrollmentID.ValueString()), q.Encode()), nil, &s); err != nil {
		resp.Diagnostics.AddError("Cannot fetch the first-boot script", err.Error())
		return
	}
	m.Content = types.StringValue(s)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
