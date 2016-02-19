package command

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/mgo.v2/bson"

	"koding/db/mongodb/modelhelper"
	"koding/kites/kloud/kloud"
	"koding/kites/kloud/utils"
	"koding/kites/kloud/utils/res"

	"github.com/koding/kite"
	"github.com/mitchellh/cli"
	"golang.org/x/net/context"
)

// Team provides an implementation for "team" command.
type Team struct {
	*res.Resource
}

// NewTeam gives new Team value.
func NewTeam() cli.CommandFactory {
	return func() (cli.Command, error) {
		f := NewFlag("team", "Plans/applies/describes/bootstraps team stacks")
		f.action = &Team{
			Resource: &res.Resource{
				Name:        "team",
				Description: "Plans/applies/describes/bootstraps team stacks",
				Commands: map[string]res.Command{
					"init":      NewTeamInit(),
					"plan":      NewTeamPlan(),
					"apply":     NewTeamApply(),
					"auth":      NewTeamAuth(),
					"bootstrap": NewTeamBootstrap(),
				},
			},
		}
		return f, nil
	}
}

func impersonate(username string, req interface{}) (v map[string]interface{}) {
	p, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(p, &v); err != nil {
		panic(err)
	}
	v["impersonate"] = username
	return v
}

// Action is an entry point for "team" subcommand.
func (t *Team) Action(args []string, k *kite.Client) error {
	ctx := context.Background()
	ctx = context.WithValue(ctx, kiteKey, k)
	modelhelper.Initialize(envMongoURL())
	t.Resource.ContextFunc = func([]string) context.Context { return ctx }
	return t.Resource.Main(args)
}

// TEAM DB

type UserOptions struct {
	Username  string
	Groupname string
	Region    string
	Provider  string
	Template  string
	KlientID  string
}

type User struct {
	MachineIDs      []bson.ObjectId
	MachineLabels   []string
	StackID         string
	StackTemplateID string
	CredID          string
	CredDataID      string
	AccountID       bson.ObjectId
	PrivateKey      string
	PublicKey       string
	Identifiers     []string
}

type TeamInit struct {
	Provider      string
	Team          string
	KlientID      string
	Username      string
	StackTemplate string
}

func NewTeamInit() *TeamInit {
	return &TeamInit{}
}

func (cmd *TeamInit) Valid() error {
	if cmd.KlientID == "" {
		return errors.New("empty value for -klient flag")
	}
	if cmd.Provider == "" {
		cmd.Provider = "vagrant"
	}
	if cmd.Provider != "vagrant" {
		return errors.New("currently only vagrant is supported")
	}
	if cmd.Team == "" {
		cmd.Team = utils.RandString(12)
	}
	if cmd.StackTemplate == "-" {
		p, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		cmd.StackTemplate = string(p)
	} else {
		p, err := ioutil.ReadFile(cmd.StackTemplate)
		if err != nil {
			return err
		}
		cmd.StackTemplate = string(p)
	}
	if cmd.StackTemplate == "" {
		return errors.New("empty value for -stack flag")
	}
	return nil
}

func (cmd *TeamInit) Name() string {
	return "init"
}

func (cmd *TeamInit) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.Provider, "p", "vagrant", "Team provider name.")
	f.StringVar(&cmd.KlientID, "klient", "", "ID of the klient kite.")
	f.StringVar(&cmd.Team, "team", "koding", "Team name. If empty will get autogenerated.")
	f.StringVar(&cmd.StackTemplate, "stack", "-", "Stack template content.")
	f.StringVar(&cmd.Username, "u", defaultUsername, "Username for the kloud request.")
}

func (cmd *TeamInit) Run(ctx context.Context) error {
	opts := &UserOptions{
		Username:  cmd.Username,
		Groupname: cmd.Team,
		Provider:  cmd.Provider,
		Template:  cmd.StackTemplate,
		KlientID:  cmd.KlientID,
	}

	user, err := CreateUser(opts)
	if err != nil {
		return err
	}

	creds := strings.Join(user.Identifiers, ",")

	var dbg string
	if flagDebug {
		dbg = " -debug"
	}

	resp := &struct {
		TeamDetails *User             `json:"teamDetails,omitempty"`
		Kloudctl    map[string]string `json:"kloudctl,omitempty"`
	}{
		TeamDetails: user,
		Kloudctl: map[string]string{
			"auth":      fmt.Sprintf("%s team%s auth -p %s -team %s -u %s -creds %s", os.Args[0], dbg, cmd.Provider, cmd.Team, cmd.Username, creds),
			"bootstrap": fmt.Sprintf("%s team%s bootstrap -p %s -team %s -u %s -creds %s", os.Args[0], dbg, cmd.Provider, cmd.Username, cmd.Team, creds),
			"plan":      fmt.Sprintf("%s team%s plan -p %s -team %s -u %s -tid %s", os.Args[0], dbg, cmd.Provider, cmd.Team, cmd.Username, user.StackTemplateID),
			"apply":     fmt.Sprintf("%s team%s apply -p %s -team %s -u %s -sid %s", os.Args[0], dbg, cmd.Provider, cmd.Team, cmd.Username, user.StackID),
		},
	}

	p, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(p))
	return nil
}

/// TEAM PLAN

// TeamPlan provides an implementation for "team plan" subcommand.
type TeamPlan struct {
	Provider        string
	Team            string
	StackTemplateID string
	Username        string
}

// NewTeamPlan gives new TeamPlan value.
func NewTeamPlan() *TeamPlan {
	return &TeamPlan{}
}

// Valid implements the kloud.Validator interface.
func (cmd *TeamPlan) Valid() error {
	if cmd.Provider == "" {
		return errors.New("empty value for -p flag")
	}
	if cmd.StackTemplateID == "" {
		return errors.New("empty value for -tid flag")
	}
	if cmd.Team == "" {
		return errors.New("empty value for -team flag")
	}
	return nil
}

// Name gives the name of the command, implements the res.Command interface.
func (cmd *TeamPlan) Name() string {
	return "plan"
}

// RegisterFlags sets the flags for the command - "team plan <flags>".
func (cmd *TeamPlan) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.Provider, "p", "aws", "Team provider name.")
	f.StringVar(&cmd.Team, "team", "koding", "Team name.")
	f.StringVar(&cmd.StackTemplateID, "tid", "", "Stack template ID.")
	f.StringVar(&cmd.Username, "u", defaultUsername, "Username for the kloud request.")
}

// Run executes the "team plan" subcommand.
func (cmd *TeamPlan) Run(ctx context.Context) error {
	k := kiteFromContext(ctx)

	req := impersonate(cmd.Username,
		&kloud.PlanRequest{
			Provider:        cmd.Provider,
			GroupName:       cmd.Team,
			StackTemplateID: cmd.StackTemplateID,
		},
	)

	resp, err := k.TellWithTimeout("plan", defaultTellTimeout, req)
	if err != nil {
		return err
	}

	DefaultUi.Info("plan raw response: " + string(resp.Raw))
	return nil
}

/// TEAM APPLY

// TeamApply provides an implementation for "team apply" subcommand.
type TeamApply struct {
	Provider string
	Team     string
	StackID  string
	Destroy  bool
	Username string
}

// NewTeamApply gives new TeamApply value.
func NewTeamApply() *TeamApply {
	return &TeamApply{}
}

// Valid implements the kloud.Validator interface.
func (cmd *TeamApply) Valid() error {
	if cmd.Provider == "" {
		return errors.New("empty value for -p flag")
	}
	if cmd.Team == "" {
		return errors.New("empty value for -team flag")
	}
	if cmd.StackID == "" {
		return errors.New("empty value for -sid flag")
	}
	return nil
}

// Name gives the name of the command, implements the res.Command interface.
func (cmd *TeamApply) Name() string {
	return "apply"
}

// RegisterFlags sets the flags for the command - "team apply <flags>".
func (cmd *TeamApply) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.Provider, "p", "aws", "Team provider name.")
	f.StringVar(&cmd.Team, "team", "koding", "Team name.")
	f.StringVar(&cmd.StackID, "sid", "", "Compute stack ID.")
	f.BoolVar(&cmd.Destroy, "del", false, "Destroy resources.")
	f.StringVar(&cmd.Username, "u", defaultUsername, "Username for the kloud request.")
}

// Run executes the "team apply" command.
func (cmd *TeamApply) Run(ctx context.Context) error {
	k := kiteFromContext(ctx)

	req := impersonate(cmd.Username,
		&kloud.ApplyRequest{
			Provider:  cmd.Provider,
			StackID:   cmd.StackID,
			GroupName: cmd.Team,
			Destroy:   cmd.Destroy,
		},
	)

	resp, err := k.TellWithTimeout("apply", defaultTellTimeout, req)
	if err != nil {
		return err
	}

	var result kloud.ControlResult
	err = resp.Unmarshal(&result)
	if err != nil {
		return err
	}

	DefaultUi.Info(fmt.Sprintf("%+v", result))

	evID := result.EventId
	if i := strings.IndexRune(evID, '-'); i != -1 {
		evID = evID[i+1:]
	}

	return watch(k, "apply", evID, defaultPollInterval)
}

/// TEAM DESCRIBE

// TeamAuth provides an implementation for "team auth" subcommand.
type TeamAuth struct {
	Provider string
	Team     string
	Creds    string
	Username string
}

// NewTeamAuth gives new TeamAuth value.
func NewTeamAuth() *TeamAuth {
	return &TeamAuth{}
}

// Valid implements the kloud.Validator interface.
func (cmd *TeamAuth) Valid() error {
	if cmd.Provider == "" {
		return errors.New("empty value for -p flag")
	}
	if cmd.Team == "" {
		return errors.New("empty value for -team flag")
	}
	if cmd.Creds == "" {
		return errors.New("empty value for -creds flag")
	}
	return nil
}

// Name gives the name of the command, implements the res.Command interface.
func (cmd *TeamAuth) Name() string {
	return "auth"
}

// RegisterFlags sets the flags for the command - "team auth <flags>".
func (cmd *TeamAuth) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.Provider, "p", "aws", "Team provider name.")
	f.StringVar(&cmd.Team, "team", "koding", "Team name.")
	f.StringVar(&cmd.Creds, "creds", "", "Comma-separated credential identifier list.")
	f.StringVar(&cmd.Username, "u", defaultUsername, "Username for the kloud request.")
}

// Run executes the "team auth" subcommand.
func (cmd *TeamAuth) Run(ctx context.Context) error {
	k := kiteFromContext(ctx)

	req := impersonate(cmd.Username,
		&kloud.AuthenticateRequest{
			Provider:    cmd.Provider,
			GroupName:   cmd.Team,
			Identifiers: strings.Split(cmd.Creds, ","),
		},
	)

	resp, err := k.TellWithTimeout("authenticate", defaultTellTimeout, req)
	if err != nil {
		return err
	}

	DefaultUi.Info("authenticate raw response: " + string(resp.Raw))
	return nil
}

/// TEAM BOOTSTRAP

// TeamBootstrap provides an implementation for "team bootstrap" subcommand.
type TeamBootstrap struct {
	Provider string
	Team     string
	Creds    string
	Destroy  bool
	Username string
}

// NewTeamBootstrap gives new TeamBootstrap value.
func NewTeamBootstrap() *TeamBootstrap {
	return &TeamBootstrap{}
}

// Valid implements the kloud.Validator interface.
func (cmd *TeamBootstrap) Valid() error {
	if cmd.Provider == "" {
		return errors.New("empty value for -p flag")
	}
	if cmd.Team == "" {
		return errors.New("empty value for -team flag")
	}
	if cmd.Creds == "" {
		return errors.New("empty value for -creds flag")
	}
	return nil
}

// Name gives the name of the command, implements the res.Command interface.
func (cmd *TeamBootstrap) Name() string {
	return "bootstrap"
}

// RegisterFlags sets the flags for the command - "team bootstrap <flags>".
func (cmd *TeamBootstrap) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.Provider, "p", "aws", "Team provider name.")
	f.StringVar(&cmd.Team, "team", "koding", "Team name.")
	f.StringVar(&cmd.Creds, "creds", "", "Comma-separated credential identifier list.")
	f.BoolVar(&cmd.Destroy, "del", false, "Destroy resources.")
	f.StringVar(&cmd.Username, "u", defaultUsername, "Username for the kloud request.")
}

// Run executes the "team bootstrap" subcommand.
func (cmd *TeamBootstrap) Run(ctx context.Context) error {
	k := kiteFromContext(ctx)

	req := impersonate(cmd.Username,
		&kloud.BootstrapRequest{
			Provider:    cmd.Provider,
			GroupName:   cmd.Team,
			Identifiers: strings.Split(cmd.Creds, ","),
			Destroy:     cmd.Destroy,
		},
	)

	resp, err := k.TellWithTimeout("bootstrap", defaultTellTimeout, req)
	if err != nil {
		return err
	}

	DefaultUi.Info("bootstrap raw response: " + string(resp.Raw))
	return nil
}
