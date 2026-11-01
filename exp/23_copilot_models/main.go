// Command 23_copilot_models lists every model the Copilot CLI offers the
// signed-in account, with its policy state, and the runtime's built-in catalog.
// Run it on a COPY of the app's Copilot home: a newer CLI may rewrite the
// config the pinned one reads. With -try, it instead asks each named model for
// one word and prints what answered and what it cost, which tells whether the
// account can use a model the list does not show.
//
//	go run ./exp/23_copilot_models -bin copilot -home /tmp/copilot-home
//	go run ./exp/23_copilot_models -bin copilot -home /tmp/copilot-home -try gpt-6.1-terra,gpt-5.6-terra
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	copilot "github.com/github/copilot-sdk/go"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
)

func main() {
	bin := flag.String("bin", "", "the copilot executable")
	home := flag.String("home", "", "a copy of the app's Copilot home")
	try := flag.String("try", "", "comma-separated model ids to ask for one word")
	flag.Parse()
	if *try != "" {
		for _, model := range strings.Split(*try, ",") {
			c := insights.New(insights.Setup{Provider: "copilot", CopilotModel: model, Copilot: *bin, CopilotHome: *home,
				Usage: func(u insights.Use) {
					fmt.Printf("%-16s answered by %s: in %d, out %d, %.4f credits, %s %s\n", model, u.Model, u.Input, u.Output, u.Credits, u.Took.Round(time.Millisecond), u.Failed)
				}})
			reply, err := c.Ask(context.Background(), "Reply with exactly one word.", "Say OK.")
			fmt.Printf("%-16s reply %q err %v\n", model, reply, err)
			c.Close()
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := copilot.NewClient(&copilot.ClientOptions{
		Connection:    copilot.StdioConnection{Path: *bin},
		BaseDirectory: *home,
		LogLevel:      "error",
		Mode:          copilot.ModeEmpty,
	})
	if err := c.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer c.Stop()
	status, err := c.GetAuthStatus(ctx)
	if err != nil || !status.IsAuthenticated {
		log.Fatalf("not signed in: %v", err)
	}
	models, err := c.ListModels(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range models {
		state, multiplier := "-", "-"
		if m.Policy != nil {
			state = m.Policy.State
		}
		if m.Billing != nil && m.Billing.Multiplier != nil {
			multiplier = fmt.Sprint(*m.Billing.Multiplier)
		}
		fmt.Printf("%-28s %-34q policy=%-12s x%s\n", m.ID, m.Name, state, multiplier)
	}
	if catalog, err := c.RPC.Models.GetBuiltInCatalog(ctx); err == nil {
		ids := []string{}
		for _, m := range catalog.Models {
			ids = append(ids, m.ID)
		}
		slices.Sort(ids)
		fmt.Printf("\nbuilt-in catalog (%d): %v\n", len(ids), ids)
	} else {
		fmt.Println("\nbuilt-in catalog:", err)
	}
}
