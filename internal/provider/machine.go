package provider

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type machineAPI struct {
	Node      string `json:"node"`
	Key       string `json:"key"`
	Scanned   bool   `json:"scanned"`
	OS        string `json:"os"`
	Compliant *bool  `json:"compliant"`
}

// ---------------------------------------------------------------- resource

type machineResource struct{ c *Client }

type machineModel struct {
	Name           types.String `tfsdk:"name"`
	WaitForJoin    types.Bool   `tfsdk:"wait_for_join"`
	JoinTimeout    types.String `tfsdk:"join_timeout"`
	RemoveOnDelete types.Bool   `tfsdk:"remove_on_destroy"`
	Key            types.String `tfsdk:"key"`
	OS             types.String `tfsdk:"os"`
}

func NewMachineResource() resource.Resource { return &machineResource{} }

func (r *machineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (r *machineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A machine's membership in the Gardience fleet. Create it next to the instance Terraform builds: it waits " +
			"until the machine has joined (so later resources can rely on it being managed), and when the instance is destroyed " +
			"it removes the machine from Gardience, so the fleet never fills with dead machines.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "The machine's name in Gardience (the `node` given to `gardience_bootstrap_script`, or its host name)."},
			"wait_for_join": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Wait until the machine has joined. Default true."},
			"join_timeout": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("20m"),
				MarkdownDescription: "How long to wait, as a duration such as `20m`."},
			"remove_on_destroy": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Remove the machine from Gardience when this resource is destroyed. Default true."},
			"key": schema.StringAttribute{Computed: true, MarkdownDescription: "The machine's key state: accepted, pending, rejected, denied or none."},
			"os":  schema.StringAttribute{Computed: true, MarkdownDescription: "Operating system, once Gardience has scanned it."},
		},
	}
}

func (r *machineResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *machineResource) get(ctx context.Context, name string) (machineAPI, error) {
	var m machineAPI
	err := r.c.Do(ctx, "GET", "/machines/"+url.PathEscape(name), nil, &m)
	return m, err
}

func (r *machineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.get(ctx, m.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot look up the machine", err.Error())
		return
	}
	if m.WaitForJoin.ValueBool() {
		d, perr := time.ParseDuration(m.JoinTimeout.ValueString())
		if perr != nil {
			resp.Diagnostics.AddAttributeError(path.Root("join_timeout"), "Bad duration", perr.Error())
			return
		}
		err = Wait(ctx, d, 10*time.Second, func() (bool, error) {
			var e error
			got, e = r.get(ctx, m.Name.ValueString())
			if e != nil {
				return false, e
			}
			if got.Key == "rejected" || got.Key == "denied" {
				return false, fmt.Errorf("the machine's key was %s by Gardience (wrong enrollment token, or a name already in use)", got.Key)
			}
			return got.Key == "accepted", nil
		})
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Machine %s did not join Gardience", m.Name.ValueString()),
				err.Error()+". Check that its user_data ran the first-boot script, that the enrollment is open, and that the machine can reach the Gardience server on ports 4505 and 4506.")
			return
		}
	}
	m.Key, m.OS = types.StringValue(got.Key), types.StringValue(got.OS)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *machineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.get(ctx, m.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the machine", err.Error())
		return
	}
	if got.Key == "none" && m.WaitForJoin.ValueBool() {
		resp.State.RemoveResource(ctx) // removed in the console: Terraform plans to wait for it again
		return
	}
	m.Key, m.OS = types.StringValue(got.Key), types.StringValue(got.OS)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *machineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m machineModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	var st machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &st)...)
	m.Key, m.OS = st.Key, st.OS
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *machineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m machineModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if !m.RemoveOnDelete.ValueBool() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/machines/"+url.PathEscape(m.Name.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot remove the machine from Gardience", err.Error())
	}
}

func (r *machineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// ---------------------------------------------------------------- data source

type machineDataSource struct{ c *Client }

type machineDSModel struct {
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	Scanned   types.Bool   `tfsdk:"scanned"`
	OS        types.String `tfsdk:"os"`
	Compliant types.Bool   `tfsdk:"compliant"`
}

func NewMachineDataSource() datasource.DataSource { return &machineDataSource{} }

func (d *machineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (d *machineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Look up a machine: whether it has joined, its operating system and whether it is compliant (patched and on baseline).",
		Attributes: map[string]dschema.Attribute{
			"name":      dschema.StringAttribute{Required: true},
			"key":       dschema.StringAttribute{Computed: true, MarkdownDescription: "accepted, pending, rejected, denied or none"},
			"scanned":   dschema.BoolAttribute{Computed: true},
			"os":        dschema.StringAttribute{Computed: true},
			"compliant": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Fully patched and on the security baseline (null until scanned)."},
		},
	}
}

func (d *machineDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	d.c = clientFrom(req.ProviderData)
}

func (d *machineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m machineDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	var got machineAPI
	if err := d.c.Do(ctx, "GET", "/machines/"+url.PathEscape(m.Name.ValueString()), nil, &got); err != nil {
		resp.Diagnostics.AddError("Cannot read the machine", err.Error())
		return
	}
	m.Key, m.Scanned, m.OS = types.StringValue(got.Key), types.BoolValue(got.Scanned), types.StringValue(got.OS)
	if got.Compliant == nil {
		m.Compliant = types.BoolNull()
	} else {
		m.Compliant = types.BoolValue(*got.Compliant)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
