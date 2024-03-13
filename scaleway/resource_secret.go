package scaleway

import (
	"context"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	secret "github.com/scaleway/scaleway-sdk-go/api/secret/v1beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/scaleway/terraform-provider-scaleway/v2/internal/httperrors"
	"github.com/scaleway/terraform-provider-scaleway/v2/internal/locality/regional"
)

func resourceScalewaySecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceScalewaySecretCreate,
		ReadContext:   resourceScalewaySecretRead,
		UpdateContext: resourceScalewaySecretUpdate,
		DeleteContext: resourceScalewaySecretDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Default: schema.DefaultTimeout(defaultSecretTimeout),
		},
		SchemaVersion: 0,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The secret name",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the secret",
			},
			"tags": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional:    true,
				Description: "List of tags [\"tag1\", \"tag2\", ...] associated to secret",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status of the secret",
			},
			"version_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The number of versions for this Secret",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Date and time of secret's creation (RFC 3339 format)",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Date and time of secret's creation (RFC 3339 format)",
			},
			"path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Location of the secret in the directory structure.",
				Default:     "/",
				DiffSuppressFunc: func(_, oldValue, newValue string, _ *schema.ResourceData) bool {
					return filepath.Clean(oldValue) == filepath.Clean(newValue)
				},
			},
			"region":     regional.Schema(),
			"project_id": projectIDSchema(),
		},
	}
}

func resourceScalewaySecretCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	api, region, err := secretAPIWithRegion(d, m)
	if err != nil {
		return diag.FromErr(err)
	}

	secretCreateRequest := &secret.CreateSecretRequest{
		Region:    region,
		ProjectID: d.Get("project_id").(string),
		Name:      d.Get("name").(string),
	}

	rawTag, tagExist := d.GetOk("tags")
	if tagExist {
		secretCreateRequest.Tags = expandStrings(rawTag)
	}

	rawDescription, descriptionExist := d.GetOk("description")
	if descriptionExist {
		secretCreateRequest.Description = expandStringPtr(rawDescription)
	}

	rawPath, pathExist := d.GetOk("path")
	if pathExist {
		secretCreateRequest.Path = expandStringPtr(rawPath)
	}

	secretResponse, err := api.CreateSecret(secretCreateRequest, scw.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(regional.NewIDString(region, secretResponse.ID))

	return resourceScalewaySecretRead(ctx, d, m)
}

func resourceScalewaySecretRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	api, region, id, err := secretAPIWithRegionAndID(m, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	secretResponse, err := api.GetSecret(&secret.GetSecretRequest{
		Region:   region,
		SecretID: id,
	}, scw.WithContext(ctx))
	if err != nil {
		if httperrors.Is404(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	if len(secretResponse.Tags) > 0 {
		_ = d.Set("tags", flattenSliceString(secretResponse.Tags))
	}

	_ = d.Set("name", secretResponse.Name)
	_ = d.Set("description", flattenStringPtr(secretResponse.Description))
	_ = d.Set("created_at", flattenTime(secretResponse.CreatedAt))
	_ = d.Set("updated_at", flattenTime(secretResponse.UpdatedAt))
	_ = d.Set("status", secretResponse.Status.String())
	_ = d.Set("version_count", int(secretResponse.VersionCount))
	_ = d.Set("region", string(region))
	_ = d.Set("project_id", secretResponse.ProjectID)
	_ = d.Set("path", secretResponse.Path)

	return nil
}

func resourceScalewaySecretUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	api, region, id, err := secretAPIWithRegionAndID(m, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	updateRequest := &secret.UpdateSecretRequest{
		Region:   region,
		SecretID: id,
	}

	hasChanged := false

	if d.HasChange("description") {
		updateRequest.Description = expandUpdatedStringPtr(d.Get("description"))
		hasChanged = true
	}

	if d.HasChange("name") {
		updateRequest.Name = expandUpdatedStringPtr(d.Get("name"))
		hasChanged = true
	}

	if d.HasChange("tags") {
		updateRequest.Tags = expandUpdatedStringsPtr(d.Get("tags"))
		hasChanged = true
	}

	if d.HasChange("path") {
		updateRequest.Path = expandUpdatedStringPtr(d.Get("path"))
		hasChanged = true
	}

	if hasChanged {
		_, err := api.UpdateSecret(updateRequest, scw.WithContext(ctx))
		if err != nil {
			return diag.FromErr(err)
		}

		if err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceScalewaySecretRead(ctx, d, m)
}

func resourceScalewaySecretDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	api, region, id, err := secretAPIWithRegionAndID(m, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = api.DeleteSecret(&secret.DeleteSecretRequest{
		Region:   region,
		SecretID: id,
	}, scw.WithContext(ctx))
	if err != nil && !httperrors.Is404(err) {
		return diag.FromErr(err)
	}

	return nil
}
