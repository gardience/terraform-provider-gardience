package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type scheduleResource struct{ c *Client }

type scheduleModel struct {
	ID      types.String            `tfsdk:"id"`
	Name    types.String            `tfsdk:"name"`
	Kind    types.String            `tfsdk:"kind"`
	Ref     types.String            `tfsdk:"ref"`
	Inputs  map[string]types.String `tfsdk:"inputs"`
	Days    []types.String          `tfsdk:"days"`
	Week    types.String            `tfsdk:"week"`
	Time    types.String            `tfsdk:"time"`
	TZ      types.String            `tfsdk:"tz"`
	Enabled types.Bool              `tfsdk:"enabled"`
	Waiting types.String            `tfsdk:"waiting"`
}

type scheduleAPI struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	Ref     string         `json:"ref"`
	Inputs  map[string]any `json:"inputs"`
	Days    []string       `json:"days"`
	Week    string         `json:"week"`
	Time    string         `json:"time"`
	TZ      string         `json:"tz"`
	Enabled bool           `json:"enabled"`
	Waiting string         `json:"waiting"`
}

func NewScheduleResource() resource.Resource { return &scheduleResource{} }

func (r *scheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule"
}

func (r *scheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A schedule: runs a runbook or workflow on set days at a set time (a maintenance window).\n\n" +
			"A schedule runs things on the fleet by itself, so one made with an API key **starts turned off**: an admin turns " +
			"it on in the console (Automation > Schedules). Changing what or when it runs, or the runbook it runs, turns it off " +
			"again until an admin looks. `enabled` shows whether it is on now. Changes that cannot be undone (wiping disks, " +
			"removing the Salt agent) are never scheduled.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Your id for the schedule: lowercase letters, digits and dashes."},
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"kind": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("runbook"),
				Validators:          []validator.String{stringvalidator.OneOf("runbook", "workflow")},
				MarkdownDescription: "`runbook` (default) or `workflow`."},
			"ref": schema.StringAttribute{Required: true, MarkdownDescription: "The runbook's or workflow's id, for example `gardience_runbook.x.id`."},
			"inputs": schema.MapAttribute{ElementType: types.StringType, Optional: true,
				MarkdownDescription: "Values for the runbook's form fields."},
			"days": schema.ListAttribute{ElementType: types.StringType, Required: true,
				MarkdownDescription: "Days it runs: `mon` to `sun`."},
			"week": schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("", "1", "2", "3", "4", "last")},
				MarkdownDescription: "Only that week of the month (`1` to `4` or `last`), for example Patch Tuesday plus a week. Unset: every week."},
			"time": schema.StringAttribute{Required: true, MarkdownDescription: "Start time, `HH:MM`."},
			"tz": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("UTC"),
				MarkdownDescription: "Time zone, for example `America/Chicago`. Default `UTC`."},
			"enabled": schema.BoolAttribute{Computed: true,
				MarkdownDescription: "Whether it is on now. Only an admin turns it on, in the console."},
			"waiting": schema.StringAttribute{Computed: true,
				MarkdownDescription: "Why it is off, when it is waiting for an admin."},
		},
	}
}

func (r *scheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *scheduleResource) put(ctx context.Context, m *scheduleModel) error {
	in := map[string]string{}
	for k, v := range m.Inputs {
		in[k] = v.ValueString()
	}
	body := map[string]any{"name": m.Name.ValueString(), "kind": m.Kind.ValueString(), "ref": m.Ref.ValueString(), "inputs": in,
		"days": strs(m.Days), "week": m.Week.ValueString(), "time": m.Time.ValueString(), "tz": m.TZ.ValueString()}
	var out scheduleAPI
	if err := r.c.Do(ctx, "PUT", "/schedules/"+url.PathEscape(m.ID.ValueString()), body, &out); err != nil {
		return err
	}
	m.Enabled, m.Waiting = types.BoolValue(out.Enabled), types.StringValue(out.Waiting)
	return nil
}

func (r *scheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot create the schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *scheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out scheduleAPI
	err := r.c.Do(ctx, "GET", "/schedules/"+url.PathEscape(m.ID.ValueString()), nil, &out)
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the schedule", err.Error())
		return
	}
	m.Name, m.Kind, m.Ref = types.StringValue(out.Name), types.StringValue(out.Kind), types.StringValue(out.Ref)
	m.Time, m.TZ, m.Week = types.StringValue(out.Time), types.StringValue(out.TZ), optStr(m.Week, out.Week)
	if !sameStrs(m.Days, out.Days) {
		m.Days = tfStrs(m.Days, out.Days)
	}
	if m.Inputs != nil || len(out.Inputs) > 0 {
		in := map[string]types.String{}
		for k, v := range out.Inputs {
			s, _ := v.(string)
			in[k] = types.StringValue(s)
		}
		m.Inputs = in
	}
	m.Enabled, m.Waiting = types.BoolValue(out.Enabled), types.StringValue(out.Waiting)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *scheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot update the schedule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *scheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/schedules/"+url.PathEscape(m.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the schedule", err.Error())
	}
}

func (r *scheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
