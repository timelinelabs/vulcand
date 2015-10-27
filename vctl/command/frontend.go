package command

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/timelinelabs/vulcand/engine"
)

func NewFrontendCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "frontend",
		Usage: "Operations with vulcan frontends",
		Subcommands: []cli.Command{
			{
				Name:   "ls",
				Usage:  "List all frontends",
				Flags:  []cli.Flag{},
				Action: cmd.printFrontendsAction,
			},
			{
				Name:  "show",
				Usage: "Show frontend details",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id"},
				},
				Action: cmd.printFrontendAction,
			},
			{
				Name:  "upsert",
				Usage: "Update or insert a frontend",
				Flags: append([]cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id, autogenerated if empty"},
					cli.StringFlag{Name: "route", Usage: "roue, will be matched against request's path"},
					cli.DurationFlag{Name: "ttl", Usage: "time to live duration, persistent if omitted"},
					cli.StringFlag{Name: "backend, b", Usage: "backend id"},
				}, frontendOptions()...),
				Action: cmd.upsertFrontendAction,
			},
			{
				Name:   "rm",
				Usage:  "Remove a frontend",
				Action: cmd.deleteFrontendAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id"},
				},
			},
		},
	}
}

func (cmd *Command) printFrontendsAction(c *cli.Context) {
	fs, err := cmd.client.GetFrontends()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printFrontends(fs)
}

func (cmd *Command) printFrontendAction(c *cli.Context) {
	fk := engine.FrontendKey{Id: c.String("id")}
	frontend, err := cmd.client.GetFrontend(fk)
	if err != nil {
		cmd.printError(err)
		return
	}

	ms, err := cmd.client.GetMiddlewares(fk)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printFrontend(frontend, ms)
}

func (cmd *Command) upsertFrontendAction(c *cli.Context) {
	settings, err := getFrontendSettings(c)
	if err != nil {
		cmd.printError(err)
		return
	}
	f, err := engine.NewHTTPFrontend(c.String("id"), c.String("b"), c.String("route"), settings)
	if err != nil {
		cmd.printError(err)
		return
	}
	if err := cmd.client.UpsertFrontend(*f, c.Duration("ttl")); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOk("frontend upserted")
}

func (cmd *Command) deleteFrontendAction(c *cli.Context) {
	err := cmd.client.DeleteFrontend(engine.FrontendKey{Id: c.String("id")})
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOk("frontend deleted")
}

func getFrontendSettings(c *cli.Context) (engine.HTTPFrontendSettings, error) {
	s := engine.HTTPFrontendSettings{}

	s.Limits.MaxMemBodyBytes = int64(c.Int("maxMemBodyKB") * 1024)
	s.Limits.MaxBodyBytes = int64(c.Int("maxBodyKB") * 1024)

	s.FailoverPredicate = c.String("failoverPredicate")
	s.Hostname = c.String("forwardHost")
	s.TrustForwardHeader = c.Bool("trustForwardHeader")
	s.PassHostHeader = c.Bool("passHostHeader")

	return s, nil
}

func frontendOptions() []cli.Flag {
	return []cli.Flag{
		// Frontend limits
		cli.IntFlag{Name: "maxMemBodyKB", Usage: "maximum request size to cache in memory, in KB"},
		cli.IntFlag{Name: "maxBodyKB", Usage: "maximum request size to allow for a frontend, in KB"},

		// Misc options
		cli.StringFlag{Name: "failoverPredicate", Usage: "predicate that defines cases when failover is allowed"},
		cli.StringFlag{Name: "forwardHost", Usage: "hostname to set when forwarding a request"},
		cli.BoolFlag{Name: "trustForwardHeader", Usage: "allows copying X-Forwarded-For header value from the original request"},
		cli.BoolFlag{Name: "passHostHeader", Usage: "allows passing custom headers to the backend servers"},
	}
}
