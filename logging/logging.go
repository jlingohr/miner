package logging

import "github.com/DistributedClocks/GoVector/govec"

func SetupLogging(process string, filename string, useTSViz bool, priority govec.LogPriority, printOnscreen bool) *govec.GoLog {
	config := govec.GetDefaultConfig()
	if useTSViz {
		config.UseTimestamps = true
	}
	config.Priority = priority
	config.PrintOnScreen = printOnscreen

	logger := govec.InitGoVector(process, filename, config)
	return logger
}
