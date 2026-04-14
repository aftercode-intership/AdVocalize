// backend/internal/jobs/monthly_reset.go
package jobs

import (
	"log"


	"github.com/robfig/cron/v3"
	"vocalize/internal/services"
)

func StartMonthlyResetJob(creditService *services.CreditService) {
	c := cron.New()

	// Run at 00:00 UTC every day
	_, err := c.AddFunc("0 0 * * *", func() {
		log.Println("Running monthly credit reset job...")
		if err := creditService.MonthlyReset(); err != nil {
			log.Printf("Monthly reset failed: %v", err)
		}
	})

	if err != nil {
		log.Printf("Failed to schedule job: %v", err)
		return
	}

	c.Start()
	log.Println("Monthly credit reset job scheduled")
}
