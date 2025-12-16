package notify

import (
	"sync"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// Notifier handles sending notifications asynchronously.
type Notifier struct {
	email       *EmailSender
	logger      *logger.Logger
	queue       chan notification
	done        chan struct{}
	wg          sync.WaitGroup
	rateLimiter *rateLimiter
}

type notification struct {
	eventType string
	data      map[string]interface{}
}

// rateLimiter prevents email flooding.
type rateLimiter struct {
	lastSent map[string]time.Time
	interval time.Duration
	mu       sync.Mutex
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{
		lastSent: make(map[string]time.Time),
		interval: interval,
	}
}

func (r *rateLimiter) allow(eventType string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	last, ok := r.lastSent[eventType]
	if !ok || time.Since(last) > r.interval {
		r.lastSent[eventType] = time.Now()
		return true
	}
	return false
}

// NewNotifier creates a new Notifier.
func NewNotifier(cfg config.NotificationConfig, log *logger.Logger) *Notifier {
	return &Notifier{
		email:       NewEmailSender(cfg, log),
		logger:      log.WithComponent("notifier"),
		queue:       make(chan notification, 100),
		done:        make(chan struct{}),
		rateLimiter: newRateLimiter(5 * time.Minute), // Max 1 email per event type per 5 minutes
	}
}

// Start begins processing notifications.
func (n *Notifier) Start() {
	n.wg.Add(1)
	go n.worker()
	n.logger.Info().Msg("Notification worker started")
}

// Stop stops the notifier and waits for pending notifications.
func (n *Notifier) Stop() {
	close(n.done)
	n.wg.Wait()
	close(n.queue)
	n.logger.Info().Msg("Notification worker stopped")
}

// worker processes notifications from the queue.
func (n *Notifier) worker() {
	defer n.wg.Done()

	for {
		select {
		case <-n.done:
			// Drain remaining notifications
			for {
				select {
				case notif := <-n.queue:
					n.send(notif)
				default:
					return
				}
			}
		case notif := <-n.queue:
			n.send(notif)
		}
	}
}

// send attempts to send a notification.
func (n *Notifier) send(notif notification) {
	// Check rate limit
	if !n.rateLimiter.allow(notif.eventType) {
		n.logger.Debug().
			Str("event", notif.eventType).
			Msg("Rate limited, skipping notification")
		return
	}

	// Check if this event type should be notified
	if !n.email.ShouldNotify(notif.eventType) {
		return
	}

	if err := n.email.SendTemplated(notif.eventType, notif.data); err != nil {
		n.logger.Error().
			Err(err).
			Str("event", notif.eventType).
			Msg("Failed to send notification")
	}
}

// Notify queues a notification for sending.
func (n *Notifier) Notify(eventType string, data map[string]interface{}) {
	select {
	case n.queue <- notification{eventType: eventType, data: data}:
	default:
		n.logger.Warn().
			Str("event", eventType).
			Msg("Notification queue full, dropping notification")
	}
}

// NotifyConnectionFailure sends a connection failure notification.
func (n *Notifier) NotifyConnectionFailure(connection, host string, err error) {
	n.Notify(config.NotifyConnectionFailure, map[string]interface{}{
		"Connection": connection,
		"Host":       host,
		"Error":      err.Error(),
	})
}

// NotifyTransferFailure sends a transfer failure notification.
func (n *Notifier) NotifyTransferFailure(fileName, rule, direction string, attempts int, err error) {
	n.Notify(config.NotifyTransferFailure, map[string]interface{}{
		"FileName":  fileName,
		"Rule":      rule,
		"Direction": direction,
		"Attempts":  attempts,
		"Error":     err.Error(),
	})
}

// NotifyServiceStart sends a service start notification.
func (n *Notifier) NotifyServiceStart(version string, ruleCount, connectionCount int) {
	n.Notify(config.NotifyServiceStart, map[string]interface{}{
		"Version":         version,
		"RuleCount":       ruleCount,
		"ConnectionCount": connectionCount,
	})
}

// NotifyServiceStop sends a service stop notification.
func (n *Notifier) NotifyServiceStop(reason string) {
	n.Notify(config.NotifyServiceStop, map[string]interface{}{
		"Reason": reason,
	})
}

// SetRateLimitInterval sets the rate limit interval.
func (n *Notifier) SetRateLimitInterval(d time.Duration) {
	n.rateLimiter.interval = d
}
