// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package codecommit

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codecommit"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_codecommit_repository", name="Repository")
// @Tags(identifierAttribute="arn")
func ResourceRepository() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceRepositoryCreate,
		UpdateWithoutTimeout: resourceRepositoryUpdate,
		ReadWithoutTimeout:   resourceRepositoryRead,
		DeleteWithoutTimeout: resourceRepositoryDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"repository_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(0, 100),
			},

			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 1000),
			},

			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"kms_key_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"repository_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"clone_url_http": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"clone_url_ssh": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"default_branch": {
				Type:     schema.TypeString,
				Optional: true,
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceRepositoryCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CodeCommitConn(ctx)

	input := &codecommit.CreateRepositoryInput{
		RepositoryName:        aws.String(d.Get("repository_name").(string)),
		RepositoryDescription: aws.String(d.Get("description").(string)),
		Tags:                  getTagsIn(ctx),
	}

	if v, ok := d.GetOk("kms_key_id"); ok {
		input.KmsKeyId = aws.String(v.(string))
	}

	out, err := conn.CreateRepositoryWithContext(ctx, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating CodeCommit Repository: %s", err)
	}

	d.SetId(d.Get("repository_name").(string))
	d.Set("repository_id", out.RepositoryMetadata.RepositoryId)
	d.Set("arn", out.RepositoryMetadata.Arn)
	d.Set("kms_key_id", out.RepositoryMetadata.KmsKeyId)
	d.Set("clone_url_http", out.RepositoryMetadata.CloneUrlHttp)
	d.Set("clone_url_ssh", out.RepositoryMetadata.CloneUrlSsh)

	if _, ok := d.GetOk("default_branch"); ok {
		if err := resourceUpdateDefaultBranch(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CodeCommit Repository (%s) default branch: %s", d.Id(), err)
		}
	}

	return append(diags, resourceRepositoryRead(ctx, d, meta)...)
}

func resourceRepositoryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CodeCommitConn(ctx)

	input := &codecommit.GetRepositoryInput{
		RepositoryName: aws.String(d.Id()),
	}

	out, err := conn.GetRepositoryWithContext(ctx, input)
	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, codecommit.ErrCodeRepositoryDoesNotExistException) {
		create.LogNotFoundRemoveState(names.CodeCommit, create.ErrActionReading, ResNameRepository, d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return create.AppendDiagError(diags, names.CodeCommit, create.ErrActionReading, ResNameRepository, d.Id(), err)
	}

	d.Set("repository_id", out.RepositoryMetadata.RepositoryId)
	d.Set("arn", out.RepositoryMetadata.Arn)
	d.Set("kms_key_id", out.RepositoryMetadata.KmsKeyId)
	d.Set("clone_url_http", out.RepositoryMetadata.CloneUrlHttp)
	d.Set("clone_url_ssh", out.RepositoryMetadata.CloneUrlSsh)
	d.Set("description", out.RepositoryMetadata.RepositoryDescription)
	d.Set("repository_name", out.RepositoryMetadata.RepositoryName)

	if _, ok := d.GetOk("default_branch"); ok {
		// The default branch can only be set when there is code in the repository
		// Preserve the configured value
		if out.RepositoryMetadata.DefaultBranch != nil { // nosemgrep:ci.helper-schema-ResourceData-Set-extraneous-nil-check
			d.Set("default_branch", out.RepositoryMetadata.DefaultBranch)
		}
	}

	return diags
}

func resourceRepositoryUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CodeCommitConn(ctx)

	if d.HasChange("repository_name") {
		if err := resourceUpdateRepositoryName(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CodeCommit Repository (%s) name: %s", d.Id(), err)
		}
	}

	if d.HasChange("default_branch") {
		if err := resourceUpdateDefaultBranch(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CodeCommit Repository (%s) default branch: %s", d.Id(), err)
		}
	}

	if d.HasChange("description") {
		if err := resourceUpdateDescription(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CodeCommit Repository (%s) description: %s", d.Id(), err)
		}
	}

	if d.HasChange("kms_key_id") {
		if err := resourceUpdateEncryptionKey(ctx, conn, d); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating CodeCommit Repository (%s) encryption key: %s", d.Id(), err)
		}
	}

	return append(diags, resourceRepositoryRead(ctx, d, meta)...)
}

func resourceRepositoryDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).CodeCommitConn(ctx)

	log.Printf("[DEBUG] CodeCommit Delete Repository: %s", d.Id())
	_, err := conn.DeleteRepositoryWithContext(ctx, &codecommit.DeleteRepositoryInput{
		RepositoryName: aws.String(d.Id()),
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting CodeCommit Repository: %s", err.Error())
	}

	return diags
}

func resourceUpdateRepositoryName(ctx context.Context, conn *codecommit.CodeCommit, d *schema.ResourceData) error {
	newName := d.Get("repository_name").(string)

	branchInput := &codecommit.UpdateRepositoryNameInput{
		OldName: aws.String(d.Id()),
		NewName: aws.String(newName),
	}

	_, err := conn.UpdateRepositoryNameWithContext(ctx, branchInput)
	if err != nil {
		return fmt.Errorf("Updating Repository Name for CodeCommit Repository: %s", err.Error())
	}

	// The Id is the name
	d.SetId(newName)

	return nil
}

func resourceUpdateDescription(ctx context.Context, conn *codecommit.CodeCommit, d *schema.ResourceData) error {
	branchInput := &codecommit.UpdateRepositoryDescriptionInput{
		RepositoryName:        aws.String(d.Id()),
		RepositoryDescription: aws.String(d.Get("description").(string)),
	}

	_, err := conn.UpdateRepositoryDescriptionWithContext(ctx, branchInput)
	if err != nil {
		return fmt.Errorf("Updating Repository Description for CodeCommit Repository: %s", err.Error())
	}

	return nil
}

func resourceUpdateDefaultBranch(ctx context.Context, conn *codecommit.CodeCommit, d *schema.ResourceData) error {
	input := &codecommit.ListBranchesInput{
		RepositoryName: aws.String(d.Id()),
	}

	out, err := conn.ListBranchesWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("reading CodeCommit Repository branches: %s", err.Error())
	}

	if len(out.Branches) == 0 {
		log.Printf("[WARN] Not setting Default Branch CodeCommit Repository that has no branches: %s", d.Id())
		return nil
	}

	branchInput := &codecommit.UpdateDefaultBranchInput{
		RepositoryName:    aws.String(d.Id()),
		DefaultBranchName: aws.String(d.Get("default_branch").(string)),
	}

	if _, err := conn.UpdateDefaultBranchWithContext(ctx, branchInput); err != nil {
		return fmt.Errorf("Updating Default Branch for CodeCommit Repository: %s", err.Error())
	}

	return nil
}

func resourceUpdateEncryptionKey(ctx context.Context, conn *codecommit.CodeCommit, d *schema.ResourceData) error {
	input := &codecommit.UpdateRepositoryEncryptionKeyInput{
		KmsKeyId:       aws.String((d.Get("kms_key_id").(string))),
		RepositoryName: aws.String(d.Id()),
	}

	_, err := conn.UpdateRepositoryEncryptionKeyWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("Updating Repository Encryption Key for CodeCommit Repository: %s", err.Error())
	}

	return nil
}
