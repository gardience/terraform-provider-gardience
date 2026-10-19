package provider

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type stateResource struct{ c *Client }

type stateModel struct {
	Name    types.String `tfsdk:"name"`
	Content types.String `tfsdk:"content"`
	SHA256  types.String `tfsdk:"sha256"`
}

type stateAPI struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

func NewStateResource() resource.Resource { return &stateResource{} }

func (r *stateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_state"
}

func (r *stateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A saved Salt state: how a machine should be (files, packages, services, users, settings). Gardience's " +
			"configuration keeper checks it before publishing (plain YAML only; no commands while command execution is off). " +
			"Saving never applies it: people apply states from the console, or a `gardience_event_rule` with `kind = \"join\"` " +
			"applies one to each new machine.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "State name: lowercase letters, digits and underscores."},
			"content": schema.StringAttribute{Required: true, MarkdownDescription: "The state, as YAML."},
			"sha256":  schema.StringAttribute{Computed: true, MarkdownDescription: "Short SHA-256 of the published file."},
		},
	}
}

func (r *stateResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	r.c = clientFrom(req.ProviderData)
}

func norm(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")) }

// save queues the state and waits until the keeper has published exactly this content.
func (r *stateResource) save(ctx context.Context, m *stateModel) error {
	name := m.Name.ValueString()
	if err := r.c.Do(ctx, "PUT", "/states/"+url.PathEscape(name), map[string]any{"content": m.Content.ValueString()}, nil); err != nil {
		return err
	}
	var got stateAPI
	err := Wait(ctx, 90*time.Second, 2*time.Second, func() (bool, error) {
		e := r.c.Do(ctx, "GET", "/states/"+url.PathEscape(name), nil, &got)
		if IsNotFound(e) {
			return false, nil
		}
		return e == nil && norm(got.Content) == norm(m.Content.ValueString()), e
	})
	if err != nil {
		return &APIError{Status: 409, Detail: "the state was not published (" + err.Error() + "). The configuration keeper refuses states " +
			"that run commands while command execution is off, use templates, or are not valid YAML; the reason is in Gardience's audit log."}
	}
	m.SHA256 = types.StringValue(got.SHA256)
	return nil
}

func (r *stateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m stateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.save(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot save the state", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *stateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m stateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got stateAPI
	err := r.c.Do(ctx, "GET", "/states/"+url.PathEscape(m.Name.ValueString()), nil, &got)
	if IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read the state", err.Error())
		return
	}
	if norm(got.Content) != norm(m.Content.ValueString()) {
		m.Content = types.StringValue(got.Content) // changed in the console: Terraform shows the drift
	}
	m.SHA256 = types.StringValue(got.SHA256)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *stateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m stateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.save(ctx, &m); err != nil {
		resp.Diagnostics.AddError("Cannot save the state", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *stateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m stateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if err := r.c.Do(ctx, "DELETE", "/states/"+url.PathEscape(m.Name.ValueString()), nil, nil); err != nil && !IsNotFound(err) {
		resp.Diagnostics.AddError("Cannot delete the state", err.Error())
	}
}

func (r *stateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
