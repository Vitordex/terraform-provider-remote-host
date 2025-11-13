// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"remote-provider/internal/provider/servers"
	"remote-provider/internal/provider/services"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RemoteFileResource{}
var _ resource.ResourceWithImportState = &RemoteFileResource{}

func NewRemoteFileResource() resource.Resource {
	return &RemoteFileResource{}
}

// RemoteFileResource defines the resource implementation.
type RemoteFileResource struct {
	sshService *services.SSHService
}

// HostConnectionModel describes the connection block attributes
type HostConnectionModel struct {
	Host       types.String `tfsdk:"host"`
	User       types.String `tfsdk:"user"`
	PrivateKey types.String `tfsdk:"private_key"`
	Password   types.String `tfsdk:"password"`
}

// ExitCodeError represents an SSH command exit code error
type ExitCodeError struct {
	code   int8
	stderr string
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d: %s", e.code, e.stderr)
}

// RemoteFileResourceModel describes the resource data model.
type RemoteFileResourceModel struct {
	Id               types.String         `tfsdk:"id"`
	HostConnection   *HostConnectionModel `tfsdk:"host_connection"`
	Path             types.String         `tfsdk:"path"`
	Content          types.String         `tfsdk:"content"`
	Privileged       types.Bool           `tfsdk:"privileged"`
	Sensitive        types.Bool           `tfsdk:"sensitive"`
	SensitiveContent types.String         `tfsdk:"sensitive_content"`
}

func (r *RemoteFileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "remote_file"
}

func (r *RemoteFileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "An existent file at a remote host",

		Attributes: map[string]schema.Attribute{
			"host_connection": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"host": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Hostname or IP address of the remote host",
					},
					"user": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "User nae to access host",
					},
					"password": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Password to access host",
					},
					"private_key": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Private key path to access host",
					},
				},
			},
			"path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Path to the file on the remote host",
			},
			"privileged": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether to run the command as root",
				Default:             booldefault.StaticBool(false),
			},
			"sensitive": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether to mark the content attribute as sensitive",
				Default:             booldefault.StaticBool(false),
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"content": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "File content",
			},
			"sensitive_content": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "File content marked as sensitive",
				Sensitive:           true,
			},
		},
	}
}

func (r *RemoteFileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	sshService, ok := req.ProviderData.(*services.SSHService)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *services.SSHService, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.sshService = sshService
}

func getFile(data *RemoteFileResourceModel, r *RemoteFileResource, ctx context.Context) error {
	server := &servers.Server{
		Address:        data.HostConnection.Host.ValueString(),
		PrivateKeyPath: data.HostConnection.PrivateKey.ValueString(),
		User:           data.HostConnection.User.ValueString(),
		Port:           22,
		Name:           data.HostConnection.Host.ValueString(),
	}
	err := r.sshService.OpenConnection(server)
	if err != nil {
		return err
	}

	// Get file inode and content using stat and cat commands
	sudoText := ""
	if data.Privileged.ValueBool() {
		sudoText = "sudo "
	}

	combinedCmd := fmt.Sprintf("%sstat -c '%%i' %s; %scat %s", sudoText, data.Path.ValueString(), sudoText, data.Path.ValueString())
	var command *servers.ServerCommand
	command, err = r.sshService.ExecuteCommand(combinedCmd, server)
	if err != nil {
		return err
	}

	if command.ExitCode != 0 {
		return &ExitCodeError{code: command.ExitCode, stderr: command.Stderr}
	}

	outputs := strings.Split(command.Stdout, "\n")

	tflog.Warn(ctx, fmt.Sprintf("outputs: %+v", command.Stdout))
	inode := strings.TrimSpace(outputs[1])
	content := strings.Join(outputs[2:], "\n")

	data.Id = types.StringValue(fmt.Sprintf("%s-%s", data.HostConnection.Host.ValueString(), inode))
	data.Content = types.StringValue("")
	data.SensitiveContent = types.StringValue("")

	if data.Sensitive.ValueBool() {
		data.SensitiveContent = types.StringValue(content)
	} else {
		data.Content = types.StringValue(content)
	}

	return nil
}

func (r *RemoteFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RemoteFileResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create example, got error: %s", err))
	//     return
	// }

	err := getFile(&data, r, ctx)
	if err != nil {
		resp.Diagnostics.AddError("SSH Error", fmt.Sprintf("Unable to execute commands, got error: %s", err))
		return
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		resp.Diagnostics.AddError("Command Error", fmt.Sprintf("Unable to get file info: %s", exitErr.Error()))
		return
	}

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RemoteFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RemoteFileResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	err := getFile(&data, r, ctx)
	if err != nil {
		resp.Diagnostics.AddError("SSH Error", fmt.Sprintf("Unable to execute commands, got error: %s", err))
		return
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		resp.Diagnostics.AddError("Command Error", fmt.Sprintf("Unable to get file info: %s", exitErr.Error()))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RemoteFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RemoteFileResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RemoteFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RemoteFileResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete example, got error: %s", err))
	//     return
	// }
}

func (r *RemoteFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
