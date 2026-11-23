package control

func OnboardingDone() bool {
	return !readDaemon().FirstRun
}
