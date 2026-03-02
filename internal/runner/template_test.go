package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJobTemplate_GoalFile(t *testing.T) {
	src := []byte(`
job {
  name = "k8s-deployments-by-namespace"
}

collector "terraform" "k8s" {
  provider    = "hashicorp/kubernetes"
  config_path = env.KUBECONFIG
}

step "terraform_datasource" "namespaces" {
  collector = collector.terraform.k8s

  datasource "kubernetes_resources" {
    api_version = "v1"
    kind        = "Namespace"
  }
}

step "terraform_datasource" "deployments" {
  collector = collector.terraform.k8s
  for_each  = step.terraform_datasource.namespaces.data.objects

  datasource "kubernetes_resources" {
    api_version = "apps/v1"
    kind        = "Deployment"
    namespace   = each.value.metadata.name
  }
}
`)

	tmpl, diags := ParseJobTemplate(src, "goal.hcl")
	require.False(t, diags.HasErrors(), "diags: %s", diags.Error())
	require.NotNil(t, tmpl)

	assert.Equal(t, "k8s-deployments-by-namespace", tmpl.JobName())
	require.Len(t, tmpl.Collectors, 1)
	assert.Equal(t, "terraform", tmpl.Collectors[0].Type)
	assert.Equal(t, "k8s", tmpl.Collectors[0].Name)

	require.Len(t, tmpl.Steps, 2)
	assert.Equal(t, "terraform_datasource", tmpl.Steps[0].Type)
	assert.Equal(t, "namespaces", tmpl.Steps[0].Name)
	assert.Nil(t, tmpl.Steps[0].ForEach, "namespaces step should not have for_each")

	assert.Equal(t, "deployments", tmpl.Steps[1].Name)
	require.NotNil(t, tmpl.Steps[1].ForEach, "deployments step should have for_each")
}

func TestParseJobTemplate_JobNameDefault(t *testing.T) {
	src := []byte(`
collector "terraform" "k8s" {
  provider = "hashicorp/kubernetes"
}
`)

	tmpl, diags := ParseJobTemplate(src, "/tmp/my-job.hcl")
	require.False(t, diags.HasErrors(), "diags: %s", diags.Error())
	assert.Equal(t, "my-job", tmpl.JobName())
}

func TestParseJobTemplate_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "duplicate collector",
			src: `
collector "terraform" "k8s" {
  provider = "a"
}

collector "terraform" "k8s" {
  provider = "b"
}`,
			wantMsg: "Duplicate collector",
		},
		{
			name: "duplicate step",
			src: `
step "static" "s" {
}

step "static" "s" {
}`,
			wantMsg: "Duplicate step",
		},
		{
			name:    "malformed HCL",
			src:     `collector "terraform" {`,
			wantMsg: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := ParseJobTemplate([]byte(tc.src), "bad.hcl")
			require.True(t, diags.HasErrors())
			if tc.wantMsg != "" {
				assert.Contains(t, diags.Error(), tc.wantMsg)
			}
		})
	}
}
