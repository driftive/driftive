package repo

import (
	"driftive/pkg/utils"
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		wantConfig *DriftiveRepoConfig
		wantErr    bool
	}{
		{
			name:     "Valid config",
			filePath: utils.GetBasePath() + "/testdata/load_config/minimal_valid_config.yaml",
			// Expects default values for unset fields
			wantConfig: &DriftiveRepoConfig{
				AutoDiscover: DriftiveRepoConfigAutoDiscover{
					Enabled: true,
					Inclusions: []string{
						"**/*.tf",
					},
				},
				GitHub: DriftiveRepoConfigGitHub{
					Issues: DriftiveRepoConfigGitHubIssues{
						MaxOpenIssues: 10,
						Errors: DriftiveRepoConfigGitHubIssuesErrors{
							MaxOpenIssues: 5,
						},
					},
					Summary: DriftiveRepoConfigGitHubSummary{
						IssueTitle: "Driftive Summary",
					},
				},
				Settings: DriftiveRepoConfigSettings{
					SkipIfOpenPR: true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadRepoConfig(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadRepoConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantConfig) {
				t.Errorf("loadRepoConfig() got = %v, want %v", got, tt.wantConfig)
			}
		})
	}
}
