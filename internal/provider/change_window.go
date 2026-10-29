package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type changeWindowResource struct{ c *Client }

type changeWindowModel struct {
	ID        types.String   `tfsdk:"id"`
	NodeGroup types.String   `tfsdk:"nodegroup"`
	Days      []types.String `tfsdk:"days"`
	Start     types.String   `tfsdk:"start"`
	End       types.String   `tfsdk:"end"`
	TZ        types.String   `tfsdk:"tz"`
}

type windowAPI struct {
	ID        string   `json:"id"`
	NodeGroup string   `json:"nodegroup"`
	Days      []string `json:"days"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	TZ        string   `json:"tz"`
}

func NewChangeWindowResource() resource.Resource { return &changeWindowResource{} }

func (r *changeWindowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_change_window"
}

func (r *changeWindowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A change window: changes to a node group (patching, states, runbooks) wait until the window is open. " +
			"Work for the group is held and runs when the window opens; an admin can override it in an emergency, with a reason " +
			"in the audit log.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Your id for the window: lowercase letters, digits and dashes."},
			"nodegroup": schema.StringAttribute{Required: true, MarkdownDescription: "The node group the window applies to."},
			"days": schema.ListAttribute{ElementType: types.StringType, Required: true,
				MarkdownDescription: "Days it is open: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`."},
			"start": schema.StringAttribute{Required: true, MarkdownDescription: "Opens at, `HH:MM`."},
			"end":   schema.StringAttribute{Required: true, MarkdownDescription: "Closes at, `HH:MM`."},
			"tz": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("UTC"),
				MarkdownDescription: "Time zone, for example `America/Chicago`. Default `UTC`."},
		},
	}
}

func (r *changeWindowResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *changeWindowResource) put(ctx context.Context, m *changeWindowModel) error {
	body := map[string]any{"nodegroup": m.NodeGroup.ValueString(), "days": strs(m.Days), "start": m.Start.ValueString(),
		"end": m.End.ValueString(), "tz": m.TZ.ValueString()}
	return r.c.Do(ctx, "PUT", "/windows/"+url.PathEscape(m.ID.ValueString()), body, nil)
}

func (r *changeWindowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m changeWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot create the change window", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *changeWindowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m changeWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out windowAPI
	err := r.c.Do(ctx, "GET", "/windows/"+url.PathEscape(m.ID.ValueString()), nil, &out)
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the change window", err.Error())
		return
	}
	m.NodeGroup, m.Start, m.End, m.TZ = types.StringValue(out.NodeGroup), types.StringValue(out.Start), types.StringValue(out.End), types.StringValue(out.TZ)
	if !sameStrs(m.Days, out.Days) {
		m.Days = tfStrs(m.Days, out.Days)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *changeWindowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m changeWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot update the change window", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *changeWindowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m changeWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/windows/"+url.PathEscape(m.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the change window", err.Error())
	}
}

func (r *changeWindowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
