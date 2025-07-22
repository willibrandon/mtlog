package mtlog

import (
	"context"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// SeqLevelController automatically updates a LoggingLevelSwitch based on 
// the minimum level configured in Seq. This enables centralized level control
// where Seq acts as the source of truth for log levels.
type SeqLevelController struct {
	levelSwitch *LoggingLevelSwitch
	seqSink     *sinks.SeqSink
	interval    time.Duration
	onError     func(error)
	
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	lastLevel  core.LogEventLevel
}

// SeqLevelControllerOptions configures a Seq level controller.
type SeqLevelControllerOptions struct {
	// CheckInterval is how often to query Seq for level changes.
	// Default: 30 seconds
	CheckInterval time.Duration
	
	// OnError is called when an error occurs during level checking.
	// Default: no-op
	OnError func(error)
	
	// InitialCheck determines whether to perform an initial check immediately.
	// Default: true
	InitialCheck bool
}

// NewSeqLevelController creates a new controller that synchronizes a level switch
// with Seq's minimum level setting. The controller will periodically query Seq
// and update the level switch when changes are detected.
func NewSeqLevelController(levelSwitch *LoggingLevelSwitch, seqSink *sinks.SeqSink, options SeqLevelControllerOptions) *SeqLevelController {
	// Apply defaults
	if options.CheckInterval <= 0 {
		options.CheckInterval = 30 * time.Second
	}
	if options.OnError == nil {
		options.OnError = func(error) {} // no-op
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	controller := &SeqLevelController{
		levelSwitch: levelSwitch,
		seqSink:     seqSink,
		interval:    options.CheckInterval,
		onError:     options.OnError,
		ctx:         ctx,
		cancel:      cancel,
		lastLevel:   levelSwitch.Level(),
	}
	
	// Start the background level checking
	controller.wg.Add(1)
	go controller.levelCheckLoop(options.InitialCheck)
	
	return controller
}

// levelCheckLoop runs the periodic level checking in a background goroutine.
func (slc *SeqLevelController) levelCheckLoop(initialCheck bool) {
	defer slc.wg.Done()
	
	// Perform initial check if requested
	if initialCheck {
		slc.checkAndUpdateLevel()
	}
	
	ticker := time.NewTicker(slc.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-slc.ctx.Done():
			return
		case <-ticker.C:
			slc.checkAndUpdateLevel()
		}
	}
}

// checkAndUpdateLevel queries Seq for the current minimum level and updates
// the level switch if it has changed.
func (slc *SeqLevelController) checkAndUpdateLevel() {
	newLevel, err := slc.seqSink.GetMinimumLevel()
	if err != nil {
		slc.onError(err)
		return
	}
	
	// Only update if the level has changed
	if newLevel != slc.lastLevel {
		slc.levelSwitch.SetLevel(newLevel)
		slc.lastLevel = newLevel
	}
}

// GetCurrentLevel returns the current level from the level switch.
func (slc *SeqLevelController) GetCurrentLevel() core.LogEventLevel {
	return slc.levelSwitch.Level()
}

// GetLastSeqLevel returns the last level retrieved from Seq.
func (slc *SeqLevelController) GetLastSeqLevel() core.LogEventLevel {
	return slc.lastLevel
}

// ForceCheck immediately queries Seq for the current level and updates
// the level switch if necessary. This is useful for testing or immediate
// synchronization.
func (slc *SeqLevelController) ForceCheck() error {
	newLevel, err := slc.seqSink.GetMinimumLevel()
	if err != nil {
		return err
	}
	
	slc.levelSwitch.SetLevel(newLevel)
	slc.lastLevel = newLevel
	return nil
}

// Close stops the level controller and waits for background operations to complete.
func (slc *SeqLevelController) Close() {
	slc.cancel()
	slc.wg.Wait()
}

// WithSeqLevelControl creates a logger with Seq-controlled dynamic level adjustment.
// This convenience function sets up both a Seq sink and automatic level control.
func WithSeqLevelControl(serverURL string, options SeqLevelControllerOptions, seqOptions ...sinks.SeqOption) (Option, *LoggingLevelSwitch, *SeqLevelController) {
	// Create Seq sink
	seqSink, err := sinks.NewSeqSink(serverURL, seqOptions...)
	if err != nil {
		panic(err) // Configuration errors should fail fast
	}
	
	// Create level switch with a reasonable default
	levelSwitch := NewLoggingLevelSwitch(core.InformationLevel)
	
	// Create level controller
	controller := NewSeqLevelController(levelSwitch, seqSink, options)
	
	// Create logger option that includes both the sink and level switch
	option := func(c *config) {
		c.sinks = append(c.sinks, seqSink)
		c.levelSwitch = levelSwitch
	}
	
	return option, levelSwitch, controller
}

// SeqLevelControllerBuilder provides a fluent interface for building Seq level controllers.
type SeqLevelControllerBuilder struct {
	serverURL     string
	options       SeqLevelControllerOptions
	seqOptions    []sinks.SeqOption
	levelSwitch   *LoggingLevelSwitch
}

// NewSeqLevelControllerBuilder creates a new builder for Seq level controllers.
func NewSeqLevelControllerBuilder(serverURL string) *SeqLevelControllerBuilder {
	return &SeqLevelControllerBuilder{
		serverURL: serverURL,
		options: SeqLevelControllerOptions{
			CheckInterval: 30 * time.Second,
			InitialCheck:  true,
		},
	}
}

// WithCheckInterval sets the interval for querying Seq.
func (b *SeqLevelControllerBuilder) WithCheckInterval(interval time.Duration) *SeqLevelControllerBuilder {
	b.options.CheckInterval = interval
	return b
}

// WithErrorHandler sets the error handler for level checking failures.
func (b *SeqLevelControllerBuilder) WithErrorHandler(onError func(error)) *SeqLevelControllerBuilder {
	b.options.OnError = onError
	return b
}

// WithInitialCheck controls whether to perform an initial level check.
func (b *SeqLevelControllerBuilder) WithInitialCheck(initialCheck bool) *SeqLevelControllerBuilder {
	b.options.InitialCheck = initialCheck
	return b
}

// WithSeqAPIKey adds API key authentication for Seq.
func (b *SeqLevelControllerBuilder) WithSeqAPIKey(apiKey string) *SeqLevelControllerBuilder {
	b.seqOptions = append(b.seqOptions, sinks.WithSeqAPIKey(apiKey))
	return b
}

// WithLevelSwitch uses an existing level switch instead of creating a new one.
func (b *SeqLevelControllerBuilder) WithLevelSwitch(levelSwitch *LoggingLevelSwitch) *SeqLevelControllerBuilder {
	b.levelSwitch = levelSwitch
	return b
}

// Build creates the Seq level controller and returns the logger option, level switch, and controller.
func (b *SeqLevelControllerBuilder) Build() (Option, *LoggingLevelSwitch, *SeqLevelController) {
	// Create Seq sink
	seqSink, err := sinks.NewSeqSink(b.serverURL, b.seqOptions...)
	if err != nil {
		panic(err)
	}
	
	// Use existing level switch or create new one
	var levelSwitch *LoggingLevelSwitch
	if b.levelSwitch != nil {
		levelSwitch = b.levelSwitch
	} else {
		levelSwitch = NewLoggingLevelSwitch(core.InformationLevel)
	}
	
	// Create controller
	controller := NewSeqLevelController(levelSwitch, seqSink, b.options)
	
	// Create logger option
	option := func(c *config) {
		c.sinks = append(c.sinks, seqSink)
		c.levelSwitch = levelSwitch
	}
	
	return option, levelSwitch, controller
}