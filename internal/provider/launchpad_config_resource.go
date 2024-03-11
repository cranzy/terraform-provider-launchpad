package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"

	mcc_mke "github.com/Mirantis/mcc/pkg/product/mke"
)

var _ resource.Resource = &LaunchpadConfigResource{}

type LaunchpadConfigResource struct {
	testingMode bool
}

func NewLaunchpadConfigResource() resource.Resource {
	return &LaunchpadConfigResource{}
}

func (r *LaunchpadConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (r *LaunchpadConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = launchpadSchema14()
}

func (r *LaunchpadConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	lpm, ok := req.ProviderData.(*LaunchpadProviderModel)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *LaunchpadProviderModel, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.testingMode = lpm.testingMode
}

func (r *LaunchpadConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	model, mke := getterToModelAndProduct(ctx, &resp.Diagnostics, req.Plan.Get, false)

	if resp.Diagnostics.HasError() {
		return
	}

	if model.SkipCreate.ValueBool() {
		resp.Diagnostics.AddWarning("skipping create", "Skipping the launchpad create because of configuration flag.")
	} else if r.testingMode {
		resp.Diagnostics.AddWarning("testing mode warning", "launchpad config resource handler is in testing mode, no installation will be run.")

	} else if err := mke.Apply(false, false, 10, false); err != nil {
		ccout, _ := yaml.Marshal(mke.ClusterConfig)
		resp.Diagnostics.AddError(
			"Launchpad apply failed",
			fmt.Sprintf("config: %s \n\n%s", ccout, err.Error()),
		)

		return
	}

	model.Id = model.Metadata.Name

	if diags := resp.State.Set(ctx, model); diags != nil {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *LaunchpadConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// launchpad has no good way to discover existing installation, so we don't do anything
}

func (r *LaunchpadConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	model, mke := getterToModelAndProduct(ctx, &resp.Diagnostics, req.Plan.Get, false)

	if resp.Diagnostics.HasError() {
		return
	}

	if model.SkipCreate.ValueBool() {
		resp.Diagnostics.AddWarning("skipping destroy", "Skipping the launchpad destroy because of configuration flag.")
	} else if r.testingMode {
		resp.Diagnostics.AddWarning("testing mode warning", "launchpad config resource handler is in testing mode, no update will be run.")

	} else if err := mke.Apply(false, false, 10, false); err != nil {
		ccout, _ := yaml.Marshal(mke.ClusterConfig)
		resp.Diagnostics.AddError(
			"Launchpad apply failed",
			fmt.Sprintf("config: %s \n\n%s", ccout, err.Error()),
		)

		return
	}

	if diags := resp.State.Set(ctx, model); diags != nil {
		resp.Diagnostics.Append(diags...)
	}
}

func (r *LaunchpadConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	model, mke := getterToModelAndProduct(ctx, &resp.Diagnostics, req.State.Get, true)

	if resp.Diagnostics.HasError() {
		return
	}

	if model.SkipDestroy.ValueBool() {
		resp.Diagnostics.AddWarning("skipping destroy", "Skipping the launchpad destroy because of configuration flag.")
	} else if r.testingMode {
		resp.Diagnostics.AddWarning("testing mode warning", "launchpad config resource handler is in testing mode, no reset will be run.")

	} else if err := mke.Reset(); err != nil {
		resp.Diagnostics.AddError(
			"Launchpad Reset failed",
			fmt.Sprintf("error: %s", err.Error()),
		)

		return
	}

	// Remove resource from state
	resp.State.RemoveResource(ctx)
}

func (r *LaunchpadConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// an import is an invalid operation for launchpad, as it will want to run anyway. Just add the resource and apply it.
	resp.Diagnostics.AddError("Launchpad imports are invalid", "The launchpad resource does not support imports, as launchpad itself doesn't maintain state. Just add the resource and hit apply.a")
}

// Get the schema model (for state) and create an MKE Product object from a getter such as a req.State.Get or req.Plan.Get or req.Config.Get
// this is a helper for frequently repeated code where we want to interpret schema into a model to add to state, and an MKE Product to take action against.
func getterToModelAndProduct(ctx context.Context, diags *diag.Diagnostics, getter func(context.Context, interface{}) diag.Diagnostics, skipValidation bool) (launchpadSchema14Model, mcc_mke.MKE) {
	var ls launchpadSchema14Model
	var mke mcc_mke.MKE

	// Read Terraform plan data into the model
	getDiags := getter(ctx, &ls)
	diags.Append(getDiags...) // this could be one-lined, but this easier to read

	if diags.HasError() {
		return ls, mke
	}

	cc := ls.ClusterConfig(diags)

	// Capture the resulting cluster config as yaml to help debugging
	if b, err := yaml.Marshal(cc); err == nil {
		tflog.Info(ctx, "ClusterConfig created", map[string]interface{}{"cc": string(b)})
	} else {
		// this is weird
		tflog.Warn(ctx, "ClusterConfig yaml Marshal failed", map[string]interface{}{"err": err})
	}

	mke = mcc_mke.MKE{ClusterConfig: cc}

	if !skipValidation {
		tflog.Debug(ctx, "running validation of created mcc.mke.clusterconfig", map[string]interface{}{})

		if err := mke.ClusterConfig.Validate(); err != nil {
			diags.AddError(
				"Launchpad config validation failed",
				err.Error(),
			)
		}
	}

	return ls, mke
}
