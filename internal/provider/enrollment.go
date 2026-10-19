package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type enrollmentResource struct{ c *Client }

type enrollmentModel struct {
	ID            types.String `tfsdk:"id"`
	Label         types.String `tfsdk:"label"`
	Hours         types.Int64  `tfsdk:"hours"`
	Groups        types.List   `tfsdk:"groups"`
	Site          types.String `tfsdk:"site"`
	Proxy         types.String `tfsdk:"proxy"`
	Token         types.String `tfsdk:"token"`
	ExpiresAt     types.String `tfsdk:"expires_at"`
	LinuxScript   types.String `tfsdk:"linux_script"`
	WindowsScript types.String `tfsdk:"windows_script"`
}

type enrollmentAPI struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Groups    []string `json:"groups"`
	Site      string   `json:"site"`
	ExpiresAt string   `json:"expires_at"`
	State     string   `json:"state"`
	Token     string   `json:"token"`
}

func NewEnrollmentResource() resource.Resource { return &enrollmentResource{} }

func (r *enrollmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enrollment"
}

func (r *enrollmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An enrollment lets new machines join Gardience without someone approving each one, until it expires " +
			"or is destroyed. Put `linux_script` or `windows_script` (or the per-machine `gardience_bootstrap_script` data source) " +
			"in a machine's `user_data`: the machine installs the pinned, hash-checked Salt agent, joins, lands in its groups and " +
			"gets its baseline from first boot. Destroying the enrollment closes it; machines that joined stay.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"label": schema.StringAttribute{Required: true, PlanModifiers: replace,
				MarkdownDescription: "What the enrollment is for, shown in the console and the audit log."},
			"hours": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(720),
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				MarkdownDescription: "How long machines may join without approval, 1 to 2160 (90 days). Default 720 (30 days). Changing it replaces the enrollment."},
			"groups": schema.ListAttribute{ElementType: types.StringType, Optional: true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
				MarkdownDescription: "Node groups every machine that joins with this enrollment belongs to."},
			"site": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replace,
				MarkdownDescription: "Site name (lowercase letters, digits, dashes); machines also join the group `site-NAME`."},
			"proxy": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), PlanModifiers: replace,
				MarkdownDescription: "A Gardience site proxy the machines connect through (`host` or `host:4505,4506`). Needs `site`."},
			"token": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep,
				MarkdownDescription: "The enrollment token (secret)."},
			"expires_at": schema.StringAttribute{Computed: true, PlanModifiers: keep, MarkdownDescription: "When it stops accepting machines (UTC)."},
			"linux_script": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep,
				MarkdownDescription: "First-boot script for Linux (POSIX sh, run as root; cloud-init or `user_data`)."},
			"windows_script": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keep,
				MarkdownDescription: "First-boot script for Windows (PowerShell as Administrator; on AWS wrap it in `<powershell></powershell>`)."},
		},
	}
}

func (r *enrollmentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *enrollmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m enrollmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groups := []string{}
	resp.Diagnostics.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
	body := map[string]any{"label": m.Label.ValueString(), "hours": m.Hours.ValueInt64(), "groups": groups,
		"site": m.Site.ValueString(), "proxy": m.Proxy.ValueString()}
	var out enrollmentAPI
	if err := r.c.Do(ctx, "POST", "/enrollments", body, &out); err != nil {
		resp.Diagnostics.AddError("Cannot create the enrollment", err.Error())
		return
	}
	m.ID, m.Token, m.ExpiresAt = types.StringValue(out.ID), types.StringValue(out.Token), types.StringValue(out.ExpiresAt)
	for _, os := range []string{"linux", "windows"} {
		var s string
		if err := r.c.Do(ctx, "GET", fmt.Sprintf("/enrollments/%s/script?os=%s", url.PathEscape(out.ID), os), nil, &s); err != nil {
			resp.Diagnostics.AddError("Cannot fetch the first-boot script", err.Error())
			return
		}
		if os == "linux" {
			m.LinuxScript = types.StringValue(s)
		} else {
			m.WindowsScript = types.StringValue(s)
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *enrollmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m enrollmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out enrollmentAPI
	err := r.c.Do(ctx, "GET", "/enrollments/"+url.PathEscape(m.ID.ValueString()), nil, &out)
	if IsNotFound(err) || (err == nil && out.State != "active") {
		resp.State.RemoveResource(ctx) // closed or expired in the console: Terraform makes a new one
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the enrollment", err.Error())
		return
	}
	m.Label, m.ExpiresAt = types.StringValue(out.Label), types.StringValue(out.ExpiresAt)
	if len(out.Groups) > 0 || !m.Groups.IsNull() {
		vals := make([]attr.Value, 0, len(out.Groups))
		for _, g := range out.Groups {
			vals = append(vals, types.StringValue(g))
		}
		if !m.Groups.IsNull() { // keep the configured order when the set is the same
			var have []string
			m.Groups.ElementsAs(ctx, &have, false)
			if sameSet(have, out.Groups) {
				vals = nil
			}
		}
		if vals != nil {
			m.Groups = types.ListValueMust(types.StringType, vals)
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *enrollmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// every argument forces a replacement; nothing to update in place
	var m enrollmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *enrollmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m enrollmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/enrollments/"+url.PathEscape(m.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot close the enrollment", err.Error())
	}
}

func (r *enrollmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddAttributeError(path.Root("id"), "Enrollments cannot be imported",
		"Their token is shown only when they are created. Create a new enrollment in Terraform instead.")
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}
