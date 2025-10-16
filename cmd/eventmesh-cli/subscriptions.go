package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newSubscriptionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Manage subscriptions",
		Long:  "List and delete explicit subscriptions",
	}

	cmd.AddCommand(newSubscriptionsListCommand())
	cmd.AddCommand(newSubscriptionsDeleteCommand())

	return cmd
}

func newSubscriptionsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all subscriptions for this client",
		Long:  "List all explicit subscriptions created by this client",
		RunE:  runSubscriptionsList,
	}

	return cmd
}

func newSubscriptionsDeleteCommand() *cobra.Command {
	var subscriptionID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a subscription",
		Long:  "Delete an explicit subscription by its ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSubscriptionsDelete(subscriptionID)
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "id", "", "Subscription ID to delete (required)")
	if err := cmd.MarkFlagRequired("id"); err != nil {
		panic(fmt.Sprintf("Failed to mark id as required: %v", err))
	}

	return cmd
}

func runSubscriptionsList(cmd *cobra.Command, args []string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Listing subscriptions for client '%s'...\n", clientID)

	subscriptions, err := client.ListSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		fmt.Println("No subscriptions found")
		return nil
	}

	fmt.Printf("\nFound %d subscription(s):\n\n", len(subscriptions))
	for i, sub := range subscriptions {
		fmt.Printf("%d. ID: %s\n", i+1, sub.ID)
		fmt.Printf("   Topic: %s\n", sub.Topic)
		fmt.Printf("   Created: %s\n", sub.CreatedAt.Format("2006-01-02 15:04:05"))
		if i < len(subscriptions)-1 {
			fmt.Println()
		}
	}

	return nil
}

func runSubscriptionsDelete(subscriptionID string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Deleting subscription '%s'...\n", subscriptionID)

	err := client.DeleteSubscription(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	fmt.Printf("✅ Subscription deleted successfully!\n")

	return nil
}