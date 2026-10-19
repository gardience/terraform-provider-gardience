package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type eventRuleResource struct{ c *Client }

type eventRuleModel struct {
	ID      types.String `tfsdk:"id"`
	Kind    types.String `tfsdk:"kind"`
	Name    types.String `tfsdk:"name"`
	OS      types.String `tfsdk:"os"`
	Service types.String `tfsdk:"service"`
	Path    types.String `tfsdk:"path"`
	Percent types.Int64  `tfsdk:"percent"`
	Scope   types.String `tfsdk:"scope"`
	Group   types.String `tfsdk:"group"`
	State   types.String `tfsdk:"state"`
}

type ruleAPI struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	OS      string `json:"os"`
	Service string `json:"service"`
	Path    string `json:"path"`
	Percent int64  `json:"percent"`
	Scope   string `json:"scope"`
	Group   string `json:"group"`
	State   string `json:"state"`
}

func NewEventRuleResource() resource.Resource { return &eventRuleResource{} }

func (r *eventRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event_rule"
}

func (r *eventRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	opt := func(d string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: d,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Event-driven automation. Each machine's Salt agent reports events as they happen, and Gardience acts in seconds:\n\n" +
			"* `service`: a watched service stopped: restart it on that machine.\n" +
			"* `disk`: a disk passed a fill level: warn (audit log and notification, at most hourly per disk).\n" +
			"* `join`: a machine joined the fleet for the first time: apply a saved state (or `desired`) to it at once. " +
			"Pair it with `gardience_enrollment` so every machine Terraform creates is configured on first boot.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Your id for the rule: letters, digits, `_` and `-`."},
			"kind": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("service", "disk", "join")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "`service`, `disk` or `join`. Changing it replaces the rule."},
			"name":    opt("Display name (a sensible default is made from the rule)."),
			"os":      schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("windows", "linux")}, MarkdownDescription: "`service` and `disk` rules: `windows` or `linux`."},
			"service": schema.StringAttribute{Optional: true, MarkdownDescription: "`service` rules: the service name, for example `W3SVC` or `nginx`."},
			"path":    schema.StringAttribute{Optional: true, MarkdownDescription: "`disk` rules: a mount point such as `/var`, or a drive such as `C:`."},
			"percent": schema.Int64Attribute{Optional: true, MarkdownDescription: "`disk` rules: fill level that triggers it, 50 to 99."},
			"scope":   schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("all", "windows", "linux", "group")}, MarkdownDescription: "`join` rules: which new machines (`all`, `windows`, `linux` or `group`)."},
			"group":   schema.StringAttribute{Optional: true, MarkdownDescription: "`join` rules with `scope = \"group\"`: the node group."},
			"state":   schema.StringAttribute{Optional: true, MarkdownDescription: "`join` rules: `desired` or the name of a `gardience_state`."},
		},
	}
}

func (r *eventRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *eventRuleResource) put(ctx context.Context, m *eventRuleModel) error {
	body := map[string]any{"kind": m.Kind.ValueString(), "os": m.OS.ValueString(), "service": m.Service.ValueString(),
		"path": m.Path.ValueString(), "scope": m.Scope.ValueString(), "group": m.Group.ValueString(), "state": m.State.ValueString()}
	if !m.Percent.IsNull() {
		body["percent"] = m.Percent.ValueInt64()
	}
	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		body["name"] = m.Name.ValueString()
	}
	var out ruleAPI
	if err := r.c.Do(ctx, "PUT", "/rules/"+url.PathEscape(m.ID.ValueString()), body, &out); err != nil {
		return err
	}
	m.Name = types.StringValue(out.Name)
	return nil
}

func (r *eventRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m eventRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot create the event rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *eventRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m eventRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out ruleAPI
	err := r.c.Do(ctx, "GET", "/rules/"+url.PathEscape(m.ID.ValueString()), nil, &out)
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the event rule", err.Error())
		return
	}
	m.Name, m.Kind = types.StringValue(out.Name), types.StringValue(out.Kind)
	set := func(cur types.String, v string) types.String {
		if cur.IsNull() && v == "" {
			return cur
		}
		return types.StringValue(v)
	}
	switch out.Kind {
	case "service":
		m.OS, m.Service = set(m.OS, out.OS), set(m.Service, out.Service)
	case "disk":
		m.OS, m.Path, m.Percent = set(m.OS, out.OS), set(m.Path, out.Path), types.Int64Value(out.Percent)
	case "join":
		m.Scope, m.Group, m.State = set(m.Scope, out.Scope), set(m.Group, out.Group), set(m.State, out.State)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *eventRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m eventRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot update the event rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *eventRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m eventRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/rules/"+url.PathEscape(m.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the event rule", err.Error())
	}
}

func (r *eventRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
