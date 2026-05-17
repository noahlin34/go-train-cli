package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/noah/go-train-cli/internal/tui"
	"github.com/noah/go-train-cli/pkg/config"
	"github.com/noah/go-train-cli/pkg/metrolinx"
	"github.com/noah/go-train-cli/pkg/output"
	"github.com/noah/go-train-cli/pkg/transit"
	"github.com/spf13/cobra"
)

type options struct {
	apiKey string
	json   bool
	about  bool
}

func NewRootCommand() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "gotrain",
		Short: "GO Transit command line utility",
		Long:  "gotrain fetches GO Transit live departures, train positions, alerts, and station data from the Metrolinx Open Data API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.about {
				fmt.Fprintln(cmd.OutOrStdout(), "Made by noah lin. All rights reserved")
				return nil
			}
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringVar(&opts.apiKey, "api-key", "", "Metrolinx GO API key (defaults to GO_API_KEY)")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "emit deterministic JSON for scripts and agents")
	root.PersistentFlags().BoolVar(&opts.about, "about", false, "show author and rights information")

	root.AddCommand(stationsCommand(opts))
	root.AddCommand(departuresCommand(opts))
	root.AddCommand(lineStopsCommand(opts))
	root.AddCommand(trainsCommand(opts))
	root.AddCommand(trainCommand(opts))
	root.AddCommand(alertsCommand(opts))
	root.AddCommand(serveCommand(opts))
	root.AddCommand(tuiCommand(opts))
	return root
}

func lineStopsCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "line-stops <line> <direction>",
		Short: "List the ordered stop topology for a GO train line",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := newService(opts).LineStops(cmd.Context(), args[0], args[1], time.Now())
			if err != nil {
				return err
			}
			if opts.json {
				return output.JSON(cmd.OutOrStdout(), snap)
			}
			rows := make([][]string, 0, len(snap.Data))
			for _, stop := range snap.Data {
				rows = append(rows, []string{
					strconv.Itoa(stop.Order),
					stop.Code,
					stop.Name,
					stop.Type,
				})
			}
			output.Rows(cmd.OutOrStdout(), []string{"ORDER", "CODE", "NAME", "TYPE"}, rows)
			return nil
		},
	}
	return cmd
}

func newService(opts *options) *transit.Service {
	cfg := config.Load(opts.apiKey)
	return transit.NewService(metrolinx.NewClient(cfg.BaseURL, cfg.APIKey))
}

func stationsCommand(opts *options) *cobra.Command {
	var trainOnly bool
	cmd := &cobra.Command{
		Use:   "stations [query]",
		Short: "List or search GO stations and stops",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			snap, err := newService(opts).Stations(cmd.Context(), query, trainOnly)
			if err != nil {
				return err
			}
			if opts.json {
				return output.JSON(cmd.OutOrStdout(), snap)
			}
			rows := make([][]string, 0, len(snap.Data))
			for _, station := range snap.Data {
				rows = append(rows, []string{station.Code, station.Name, station.Type})
			}
			output.Rows(cmd.OutOrStdout(), []string{"CODE", "NAME", "TYPE"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&trainOnly, "train-only", true, "only include train stations")
	return cmd
}

func departuresCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "departures <station-code>",
		Aliases: []string{"next"},
		Short:   "Fetch live departure payload for a station",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := newService(opts).Departures(cmd.Context(), strings.ToUpper(args[0]))
			if err != nil {
				return err
			}
			return output.JSON(cmd.OutOrStdout(), snap)
		},
	}
	return cmd
}

func trainsCommand(opts *options) *cobra.Command {
	var line string
	cmd := &cobra.Command{
		Use:     "trains",
		Aliases: []string{"line", "track"},
		Short:   "List live train positions, optionally filtered by line",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && line == "" {
				line = args[0]
			}
			snap, err := newService(opts).Trains(cmd.Context(), line)
			if err != nil {
				return err
			}
			if opts.json {
				return output.JSON(cmd.OutOrStdout(), snap)
			}
			rows := make([][]string, 0, len(snap.Data))
			for _, train := range snap.Data {
				rows = append(rows, []string{
					train.Line,
					train.TripNumber,
					train.Display,
					train.PositionLabel,
					formatDelay(train.DelaySeconds),
					train.LastUpdated,
				})
			}
			output.Rows(cmd.OutOrStdout(), []string{"LINE", "TRIP", "DISPLAY", "POSITION", "DELAY", "UPDATED"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVarP(&line, "line", "l", "", "filter by GO line code, e.g. LW")
	return cmd
}

func trainCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "train <trip-number>",
		Short: "Show one live train by trip number",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := newService(opts).Train(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if opts.json {
				return output.JSON(cmd.OutOrStdout(), snap)
			}
			if snap.Data == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "No live train found for trip %s\n", args[0])
				return nil
			}
			train := snap.Data
			output.Rows(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, [][]string{
				{"Line", train.Line},
				{"Trip", train.TripNumber},
				{"Display", train.Display},
				{"Position", train.PositionLabel},
				{"Delay", formatDelay(train.DelaySeconds)},
				{"Last updated", train.LastUpdated},
			})
			return nil
		},
	}
	return cmd
}

func alertsCommand(opts *options) *cobra.Command {
	var line string
	cmd := &cobra.Command{
		Use:     "alerts",
		Aliases: []string{"status"},
		Short:   "List active service alerts",
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := newService(opts).Alerts(cmd.Context(), line)
			if err != nil {
				return err
			}
			if opts.json {
				return output.JSON(cmd.OutOrStdout(), snap)
			}
			rows := make([][]string, 0, len(snap.Data))
			for _, alert := range snap.Data {
				rows = append(rows, []string{
					alert.Status,
					strings.Join(alert.Lines, ","),
					alert.Subject,
					alert.PostedAt,
				})
			}
			output.Rows(cmd.OutOrStdout(), []string{"STATUS", "LINES", "SUBJECT", "POSTED"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVarP(&line, "line", "l", "", "filter by GO line code, e.g. LW")
	return cmd
}

func serveCommand(opts *options) *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve gotrain data over a small JSON HTTP API",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newService(opts)
			mux := http.NewServeMux()
			mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
				_ = output.JSON(w, map[string]string{"status": "ok"})
			})
			mux.HandleFunc("GET /stations", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.Stations(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("train_only") != "false")
				writeHTTP(w, snap, err)
			})
			mux.HandleFunc("GET /departures/{station}", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.Departures(r.Context(), r.PathValue("station"))
				writeHTTP(w, snap, err)
			})
			mux.HandleFunc("GET /line-stops/{line}/{direction}", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.LineStops(r.Context(), r.PathValue("line"), r.PathValue("direction"), time.Now())
				writeHTTP(w, snap, err)
			})
			mux.HandleFunc("GET /trains", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.Trains(r.Context(), r.URL.Query().Get("line"))
				writeHTTP(w, snap, err)
			})
			mux.HandleFunc("GET /trains/{trip}", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.Train(r.Context(), r.PathValue("trip"))
				writeHTTP(w, snap, err)
			})
			mux.HandleFunc("GET /alerts", func(w http.ResponseWriter, r *http.Request) {
				snap, err := svc.Alerts(r.Context(), r.URL.Query().Get("line"))
				writeHTTP(w, snap, err)
			})
			fmt.Fprintf(cmd.OutOrStdout(), "gotrain serving JSON on http://%s\n", addr)
			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "listen address")
	return cmd
}

func tuiCommand(opts *options) *cobra.Command {
	var line string
	var refresh int
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open an interactive live GO train dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := newService(opts)
			return tui.Run(context.Background(), svc, strings.ToUpper(line), refresh, os.Stdout)
		},
	}
	cmd.Flags().StringVarP(&line, "line", "l", "LW", "initial GO line code")
	cmd.Flags().IntVar(&refresh, "refresh", 20, "refresh interval in seconds")
	return cmd
}

func writeHTTP(w http.ResponseWriter, v any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = output.JSON(w, map[string]string{"error": err.Error()})
		return
	}
	_ = output.JSON(w, v)
}

func formatDelay(seconds int) string {
	if seconds == 0 {
		return "on time"
	}
	minutes := delayMinutes(seconds)
	unit := "minutes"
	if minutes == 1 {
		unit = "minute"
	}
	if seconds < 0 {
		return "delayed by " + strconv.Itoa(minutes) + " " + unit
	}
	return "early by " + strconv.Itoa(minutes) + " " + unit
}

func delayMinutes(seconds int) int {
	if seconds < 0 {
		seconds = -seconds
	}
	minutes := seconds / 60
	if seconds%60 != 0 {
		minutes++
	}
	return minutes
}
