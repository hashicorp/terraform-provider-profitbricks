package profitbricks

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/profitbricks/profitbricks-sdk-go"
)

// Provider returns a schema.Provider for ProfitBricks.
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"username": {
				Type:          schema.TypeString,
				Optional:      true,
				DefaultFunc:   schema.EnvDefaultFunc("PROFITBRICKS_USERNAME", nil),
				Description:   "ProfitBricks username for API operations. If token is provided, token is prefered",
				ConflictsWith: []string{"token"},
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				DefaultFunc:   schema.EnvDefaultFunc("PROFITBRICKS_PASSWORD", nil),
				Description:   "ProfitBricks password for API operations. If token is provided, token is prefered",
				ConflictsWith: []string{"token"},
			},
			"token": {
				Type:          schema.TypeString,
				Optional:      true,
				DefaultFunc:   schema.EnvDefaultFunc("PROFITBRICKS_TOKEN", ""),
				Description:   "Profitbricks bearer token for API operations.",
				ConflictsWith: []string{"username", "password"},
			},
			"endpoint": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PROFITBRICKS_API_URL", ""),
				Description: "ProfitBricks REST API URL.",
			},
			"retries": {
				Type:       schema.TypeInt,
				Optional:   true,
				Default:    50,
				Deprecated: "Timeout is used instead of this functionality",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"profitbricks_datacenter":   resourceProfitBricksDatacenter(),
			"profitbricks_ipblock":      resourceProfitBricksIPBlock(),
			"profitbricks_firewall":     resourceProfitBricksFirewall(),
			"profitbricks_lan":          resourceProfitBricksLan(),
			"profitbricks_loadbalancer": resourceProfitBricksLoadbalancer(),
			"profitbricks_nic":          resourceProfitBricksNic(),
			"profitbricks_server":       resourceProfitBricksServer(),
			"profitbricks_volume":       resourceProfitBricksVolume(),
			"profitbricks_group":        resourceProfitBricksGroup(),
			"profitbricks_share":        resourceProfitBricksShare(),
			"profitbricks_user":         resourceProfitBricksUser(),
			"profitbricks_snapshot":     resourceProfitBricksSnapshot(),
			"profitbricks_ipfailover":   resourceProfitBricksLanIPFailover(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"profitbricks_datacenter": dataSourceDataCenter(),
			"profitbricks_location":   dataSourceLocation(),
			"profitbricks_image":      dataSourceImage(),
			"profitbricks_resource":   dataSourceResource(),
			"profitbricks_snapshot":   dataSourceSnapshot(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	username, usernameOk := d.GetOk("username")
	password, passwordOk := d.GetOk("password")
	token, _ := d.GetOk("token")

	if token == "" {
		if !usernameOk {
			return nil, fmt.Errorf("Neither ProfitBricks token, nor ProfitBricks username has been provided")
		}

		if !passwordOk {
			return nil, fmt.Errorf("Neither ProfitBricks token, nor ProfitBricks password has been provided")
		}
	} else {
		if usernameOk || passwordOk {
			return nil, fmt.Errorf("Only provide ProfitBricks token OR ProfitBricks username/password.")
		}
	}

	config := Config{
		Username: username.(string),
		Password: password.(string),
		Endpoint: cleanURL(d.Get("endpoint").(string)),
		Retries:  d.Get("retries").(int),
		Token:    token.(string),
	}

	return config.Client()
}

// cleanURL makes sure trailing slash does not corrupte the state
func cleanURL(url string) string {
	length := len(url)
	if length > 1 && url[length-1] == '/' {
		url = url[:length-1]
	}

	return url
}

// getStateChangeConf gets the default configuration for tracking a request progress
func getStateChangeConf(meta interface{}, d *schema.ResourceData, location string, timeoutType string) *resource.StateChangeConf {
	stateConf := &resource.StateChangeConf{
		Pending:        resourcePendingStates,
		Target:         resourceTargetStates,
		Refresh:        resourceStateRefreshFunc(meta, location),
		Timeout:        d.Timeout(timeoutType),
		MinTimeout:     10 * time.Second,
		Delay:          10 * time.Second, // Wait 10 secs before starting
		NotFoundChecks: 600,              //Setting high number, to support long timeouts
	}

	return stateConf
}

// resourceStateRefreshFunc tracks progress of a request
func resourceStateRefreshFunc(meta interface{}, path string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		client := meta.(*profitbricks.Client)

		if path == "" {
			return nil, "", fmt.Errorf("Can not check a state when path is empty")
		}

		request, err := client.GetRequestStatus(path)

		if err != nil {
			return nil, "", fmt.Errorf("Request failed with following error: %s", err)
		}

		if request.Metadata.Status == "FAILED" {
			return nil, "", fmt.Errorf("Request failed with following error: %s", request.Metadata.Message)
		}

		if request.Metadata.Status == "DONE" {
			return request, "DONE", nil
		}

		return nil, request.Metadata.Status, nil
	}
}

// resourcePendingStates defines states of working in progress
var resourcePendingStates = []string{
	"RUNNING",
	"QUEUED",
}

// resourceTargetStates defines states of completion
var resourceTargetStates = []string{
	"DONE",
}

// resourceDefaultTimeouts sets default value for each Timeout type
var resourceDefaultTimeouts = schema.ResourceTimeout{
	Create:  schema.DefaultTimeout(60 * time.Minute),
	Update:  schema.DefaultTimeout(60 * time.Minute),
	Delete:  schema.DefaultTimeout(60 * time.Minute),
	Default: schema.DefaultTimeout(60 * time.Minute),
}
