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

type runbookResource struct{ c *Client }

type surveyModel struct {
	Name    types.String   `tfsdk:"name"`
	Label   types.String   `tfsdk:"label"`
	Type    types.String   `tfsdk:"type"`
	Options []types.String `tfsdk:"options"`
	Default types.String   `tfsdk:"default"`
}

type runbookModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Fun         types.String   `tfsdk:"fun"`
	Args        []types.String `tfsdk:"args"`
	Target      types.String   `tfsdk:"target"`
	TargetType  types.String   `tfsdk:"target_type"`
	Survey      []surveyModel  `tfsdk:"survey"`
}

type surveyAPI struct {
	Name    string   `json:"name"`
	Label   string   `json:"label"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
	Default string   `json:"default"`
}

type runbookAPI struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Fun         string      `json:"fun"`
	Args        []string    `json:"args"`
	Target      string      `json:"target"`
	TargetType  string      `json:"target_type"`
	Survey      []surveyAPI `json:"survey"`
}

func NewRunbookResource() resource.Resource { return &runbookResource{} }

func (r *runbookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook"
}

func (r *runbookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A runbook: one Salt function with a form, that people run from the console (and schedules run on " +
			"their days). Saving never runs it. Only functions the server's automation identity may run are accepted; root on " +
			"the server decides that list. Values in `args` and `target` can use the form's fields, as `{{name}}`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Your id for the runbook: lowercase letters, digits and dashes."},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description": schema.StringAttribute{Optional: true, MarkdownDescription: "What it is for."},
			"fun":         schema.StringAttribute{Required: true, MarkdownDescription: "The Salt function, for example `service.restart`."},
			"args": schema.ListAttribute{ElementType: types.StringType, Optional: true,
				MarkdownDescription: "Arguments, for example `[\"{{service}}\"]`."},
			"target": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("*"),
				MarkdownDescription: "Which machines: a node group name, a list, or a pattern. Can use a form field."},
			"target_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("glob"),
				Validators:          []validator.String{stringvalidator.OneOf("glob", "list", "nodegroup", "compound")},
				MarkdownDescription: "`glob` (default), `list`, `nodegroup` or `compound`."},
			"survey": schema.ListNestedAttribute{Optional: true, MarkdownDescription: "The form people fill in when they run it.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":  schema.StringAttribute{Required: true, MarkdownDescription: "Field name, used as `{{name}}`."},
					"label": schema.StringAttribute{Optional: true, MarkdownDescription: "Label shown on the form (default: the name)."},
					"type": schema.StringAttribute{Optional: true,
						Validators:          []validator.String{stringvalidator.OneOf("text", "number", "select", "nodegroup", "project", "patchset")},
						MarkdownDescription: "`text` (default), `number`, `select`, `nodegroup`, `project` or `patchset`."},
					"options": schema.ListAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: "Choices for `select`."},
					"default": schema.StringAttribute{Optional: true, MarkdownDescription: "Value filled in at first."},
				}}},
		},
	}
}

func (r *runbookResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func (r *runbookResource) put(ctx context.Context, m *runbookModel) error {
	sv := []surveyAPI{}
	for _, f := range m.Survey {
		sv = append(sv, surveyAPI{Name: f.Name.ValueString(), Label: f.Label.ValueString(), Type: f.Type.ValueString(),
			Options: strs(f.Options), Default: f.Default.ValueString()})
	}
	body := map[string]any{"name": m.Name.ValueString(), "description": m.Description.ValueString(), "fun": m.Fun.ValueString(),
		"args": strs(m.Args), "target": m.Target.ValueString(), "target_type": m.TargetType.ValueString(), "survey": sv}
	return r.c.Do(ctx, "PUT", "/runbooks/"+url.PathEscape(m.ID.ValueString()), body, nil)
}

// sameSurvey compares the form as written with the form as stored (the server fills in a label and a type).
func sameSurvey(a []surveyModel, b []surveyAPI) bool {
	if len(a) != len(b) {
		return false
	}
	for i, f := range a {
		label, typ := f.Label.ValueString(), f.Type.ValueString()
		if label == "" {
			label = f.Name.ValueString()
		}
		if typ == "" {
			typ = "text"
		}
		if f.Name.ValueString() != b[i].Name || label != b[i].Label || typ != b[i].Type || f.Default.ValueString() != b[i].Default ||
			!sameStrs(f.Options, b[i].Options) {
			return false
		}
	}
	return true
}

func (r *runbookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m runbookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot create the runbook", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *runbookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m runbookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out runbookAPI
	err := r.c.Do(ctx, "GET", "/runbooks/"+url.PathEscape(m.ID.ValueString()), nil, &out)
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the runbook", err.Error())
		return
	}
	m.Name, m.Fun = types.StringValue(out.Name), types.StringValue(out.Fun)
	m.Description = optStr(m.Description, out.Description)
	m.Target, m.TargetType = types.StringValue(out.Target), types.StringValue(out.TargetType)
	if !sameStrs(m.Args, out.Args) {
		m.Args = tfStrs(m.Args, out.Args)
	}
	if !sameSurvey(m.Survey, out.Survey) { // changed in the console: Terraform shows the drift
		m.Survey = nil
		for _, f := range out.Survey {
			m.Survey = append(m.Survey, surveyModel{Name: types.StringValue(f.Name), Label: types.StringValue(f.Label),
				Type: types.StringValue(f.Type), Options: tfStrs(nil, f.Options), Default: optStr(types.StringNull(), f.Default)})
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *runbookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m runbookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot update the runbook", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *runbookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m runbookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/runbooks/"+url.PathEscape(m.ID.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the runbook", err.Error())
	}
}

func (r *runbookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
