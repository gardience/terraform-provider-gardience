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

type nodeGroupResource struct{ c *Client }

type nodeGroupModel struct {
	Name    types.String `tfsdk:"name"`
	Kind    types.String `tfsdk:"kind"`
	Nodes   types.List   `tfsdk:"nodes"`
	OS      types.String `tfsdk:"os"`
	Pattern types.String `tfsdk:"pattern"`
	Expr    types.String `tfsdk:"expr"`
	Target  types.String `tfsdk:"target"`
	Members types.List   `tfsdk:"members"`
}

func NewNodeGroupResource() resource.Resource { return &nodeGroupResource{} }

func (r *nodeGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_group"
}

func (r *nodeGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A custom node group: targets for runbooks, schedules, change windows, the secrets vault and join rules.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Group name: letters, numbers, spaces, dots, dashes or underscores (up to 60)."},
			"kind": schema.StringAttribute{Required: true,
				Validators:          []validator.String{stringvalidator.OneOf("nodes", "os", "match", "compound")},
				MarkdownDescription: "How members are chosen: `nodes` (a list), `os`, `match` (a name pattern) or `compound` (a Salt compound expression)."},
			"nodes":   schema.ListAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: "For `kind = \"nodes\"`: machine names."},
			"os":      schema.StringAttribute{Optional: true, MarkdownDescription: "For `kind = \"os\"`: Windows, Linux, RHEL family, Debian family or SUSE family."},
			"pattern": schema.StringAttribute{Optional: true, MarkdownDescription: "For `kind = \"match\"`: a name pattern such as `web-*`."},
			"expr":    schema.StringAttribute{Optional: true, MarkdownDescription: "For `kind = \"compound\"`: a Salt compound expression."},
			"target": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The Salt expression Gardience stores for the group."},
			"members": schema.ListAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Machines currently in the group."},
		},
	}
}

func (r *nodeGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

type groupAPI struct {
	Name    string   `json:"name"`
	Source  string   `json:"source"`
	Expr    string   `json:"expr"`
	Members []string `json:"members"`
}

func (r *nodeGroupResource) put(ctx context.Context, m *nodeGroupModel) error {
	body := map[string]any{"kind": m.Kind.ValueString(), "os": m.OS.ValueString(), "pattern": m.Pattern.ValueString(), "expr": m.Expr.ValueString()}
	nodes := []string{}
	m.Nodes.ElementsAs(ctx, &nodes, false)
	body["nodes"] = nodes
	if err := r.c.Do(ctx, "PUT", "/groups/"+url.PathEscape(m.Name.ValueString()), body, nil); err != nil {
		return err
	}
	return r.read(ctx, m)
}

func (r *nodeGroupResource) read(ctx context.Context, m *nodeGroupModel) error {
	var g groupAPI
	if err := r.c.Do(ctx, "GET", "/groups/"+url.PathEscape(m.Name.ValueString()), nil, &g); err != nil {
		return err
	}
	m.Target = types.StringValue(g.Expr)
	if g.Members == nil {
		g.Members = []string{}
	}
	ms, _ := types.ListValueFrom(ctx, types.StringType, g.Members)
	m.Members = ms
	return nil
}

func (r *nodeGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m nodeGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot create the node group", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *nodeGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m nodeGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.read(ctx, &m); IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	} else if err != nil {
		resp.Diagnostics.AddError("Cannot read the node group", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *nodeGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m nodeGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot update the node group", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *nodeGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m nodeGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/groups/"+url.PathEscape(m.Name.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the node group", err.Error())
	}
}

func (r *nodeGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
