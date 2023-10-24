// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-provider-vault/internal/consts"
)

func MustAddSchema(r *schema.Resource, m map[string]*schema.Schema) {
	for k, s := range m {
		mustAddSchema(k, s, r.Schema)
	}
}

func MustAddSchemaResource(s map[string]*schema.Resource, d map[string]*schema.Resource,
	f func(r *schema.Resource) *schema.Resource,
) {
	for n, r := range s {
		if f != nil {
			r = f(r)
		}
		if _, ok := d[n]; ok {
			panic(fmt.Sprintf("cannot add resource, Resource map already contains %q", n))
		}
		d[n] = r
	}
}

func mustAddSchema(k string, s *schema.Schema, d map[string]*schema.Schema) {
	if _, ok := d[k]; ok {
		panic(fmt.Sprintf("cannot add schema field %q,  already exists in the Schema map", k))
	}

	d[k] = s
}

func MustAddMountMigrationSchema(r *schema.Resource, customStateUpgrade bool) *schema.Resource {
	MustAddSchema(r, map[string]*schema.Schema{
		consts.FieldDisableRemount: {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  false,
			Description: "If set, opts out of mount migration " +
				"on path updates.",
		},
	})

	if !customStateUpgrade {
		// Enable disable_remount default state upgrade
		// Since we are adding a new boolean parameter that is expected
		// to be set to a default upon upgrading, we update the TF state
		// and set disable_remount to 'false' ONLY if it was previously 'nil'
		//
		// This case should only occur when upgrading from a version that
		// does not support the disable_remount parameter (<v3.9.0)
		r.StateUpgraders = defaultDisableRemountStateUpgraders()
		r.SchemaVersion = 1
	}

	return r
}

func NamespacePathCustomizeDiffFunc() schema.CustomizeDiffFunc {
	return func(ctx context.Context, diff *schema.ResourceDiff, meta interface{}) error {
		field := consts.FieldNamespace
		if !diff.HasChange(field) {
			return nil
		}

		o, n := diff.GetChange(field)
		// if namespace is just set, ignore
		if o == "" {
			return nil
		}

		// get namespace set in provider block
		client := meta.(*ProviderMeta).GetClient()
		parentNS := client.Namespace()

		// construct provider block ns + resource block ns path
		constructedNamespace := fmt.Sprintf("%s/%s", parentNS, n)

		fmt.Printf("[VINAY]: parent NS: %s\n", parentNS)
		fmt.Printf("[VINAY]: constructed NS: %s\n", constructedNamespace)
		fmt.Printf("[VINAY]: old NS in TF state: %s\n", o)
		fmt.Printf("[VINAY]: new NS in TF config: %s\n", n)

		// compare fully qualified path in TF state with new constructed namespace
		if o.(string) == constructedNamespace {
			return nil
		}

		// New NS path does not match existing namespace in state
		// Destroy and recreate resource
		return diff.ForceNew(field)
	}
}

func GetNamespaceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		consts.FieldNamespace: {
			Type:         schema.TypeString,
			Optional:     true,
			ForceNew:     true,
			Description:  "Target namespace. (requires Enterprise)",
			ValidateFunc: ValidateNoLeadingTrailingSlashes,
		},
	}
}

func MustAddNamespaceSchema(d map[string]*schema.Schema) {
	for k, s := range GetNamespaceSchema() {
		mustAddSchema(k, s, d)
	}
}

func SecretsAuthMountDisableRemountResourceV0() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			consts.FieldDisableRemount: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "If set, opts out of mount migration " +
					"on path updates.",
			},
		},
	}
}

func SecretsAuthMountDisableRemountUpgradeV0(
	_ context.Context, rawState map[string]interface{}, _ interface{},
) (map[string]interface{}, error) {
	if rawState[consts.FieldDisableRemount] == nil {
		rawState[consts.FieldDisableRemount] = false
	}

	return rawState, nil
}

func defaultDisableRemountStateUpgraders() []schema.StateUpgrader {
	return []schema.StateUpgrader{
		{
			Version: 0,
			Type:    SecretsAuthMountDisableRemountResourceV0().CoreConfigSchema().ImpliedType(),
			Upgrade: SecretsAuthMountDisableRemountUpgradeV0,
		},
	}
}
