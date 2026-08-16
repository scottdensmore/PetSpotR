package notification

import (
	"context"
	"fmt"
	"log"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// CommunitySubscriber represents a registered community member seeking neighborhood alerts.
type CommunitySubscriber struct {
	ID          string               `json:"id"`
	Email       string               `json:"email"`
	Phone       string               `json:"phone"`
	Coordinates domain.LocationPoint `json:"coordinates"`
	RadiusMiles float64              `json:"radiusMiles"`
	Channels    []Channel            `json:"channels"`
}

// GeoBroadcastEngine handles radius-based community broadcast alerts.
type GeoBroadcastEngine struct {
	subscribers []CommunitySubscriber
	dispatcher  *MultiChannelDispatcher
}

type notificationDispatch func(context.Context, *NotificationMessage) ([]DispatchResult, error)

// NewGeoBroadcastEngine constructs a GeoBroadcastEngine instance.
func NewGeoBroadcastEngine(subscribers []CommunitySubscriber, dispatcher *MultiChannelDispatcher) *GeoBroadcastEngine {
	return &GeoBroadcastEngine{
		subscribers: subscribers,
		dispatcher:  dispatcher,
	}
}

// DefaultSubscribers provides a pre-populated list of sample community subscribers for Seattle area.
func DefaultSubscribers() []CommunitySubscriber {
	return []CommunitySubscriber{
		{
			ID:          "sub-capitol-hill",
			Email:       "capitolhill.resident@example.com",
			Phone:       "+12065550111",
			Coordinates: domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200},
			RadiusMiles: 5.0,
			Channels:    []Channel{ChannelEmail, ChannelSMS},
		},
		{
			ID:          "sub-green-lake",
			Email:       "greenlake.resident@example.com",
			Phone:       "+12065550222",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5.0,
			Channels:    []Channel{ChannelEmail, ChannelPush},
		},
	}
}

// BroadcastLostPetAlert computes distance between lost pet location and subscribers, broadcasting urgent alerts to nearby residents.
func (g *GeoBroadcastEngine) BroadcastLostPetAlert(ctx context.Context, evt *domain.LostPetReportedV2, maxRadiusMiles float64) ([]DispatchResult, error) {
	return g.broadcastLostPetAlert(ctx, evt, maxRadiusMiles, g.dispatcher.Dispatch)
}

func (g *GeoBroadcastEngine) broadcastLostPetAlert(
	ctx context.Context,
	evt *domain.LostPetReportedV2,
	maxRadiusMiles float64,
	dispatch notificationDispatch,
) ([]DispatchResult, error) {
	if evt == nil {
		return nil, fmt.Errorf("evt cannot be nil")
	}
	if dispatch == nil {
		return nil, fmt.Errorf("notification dispatch cannot be nil")
	}
	if evt.GeocodingStatus != domain.GeocodingVerified {
		log.Printf("[GEO BROADCAST] Skipping lost pet %s because coordinates are not verified", evt.PetID)
		return []DispatchResult{}, nil
	}
	if evt.Coordinates == nil {
		return nil, fmt.Errorf("verified lost pet %s has no coordinates", evt.PetID)
	}
	if err := evt.Coordinates.Validate(); err != nil {
		return nil, fmt.Errorf("lost pet %s has invalid coordinates: %w", evt.PetID, err)
	}

	petPoint := *evt.Coordinates
	allResults := make([]DispatchResult, 0)

	log.Printf("[GEO BROADCAST] Processing lost pet %s near %s (Lat: %.4f, Lon: %.4f)",
		evt.PetID, evt.Location, petPoint.Latitude, petPoint.Longitude)

	for _, sub := range g.subscribers {
		distMiles := domain.HaversineDistanceMiles(petPoint, sub.Coordinates)

		// Check if subscriber is within requested radius and their own preference radius
		effectiveRadius := maxRadiusMiles
		if sub.RadiusMiles < effectiveRadius {
			effectiveRadius = sub.RadiusMiles
		}

		if distMiles <= effectiveRadius {
			log.Printf("[GEO BROADCAST] Subscriber %s is within %.2f miles (Threshold: %.2f mi). Dispatching alert...",
				sub.ID, distMiles, effectiveRadius)

			msg := &NotificationMessage{
				RecipientID: sub.ID,
				Email:       sub.Email,
				Phone:       sub.Phone,
				PushToken:   fmt.Sprintf("token-%s", sub.ID),
				Subject:     fmt.Sprintf("🚨 URGENT: Lost Pet Alert in Your Neighborhood (%s)", evt.Location),
				Body:        fmt.Sprintf("A pet (%s) was reported lost near %s (approx %.1f miles from you). Please keep an eye out!", evt.PetID, evt.Location, distMiles),
				Channels:    sub.Channels,
			}

			results, err := dispatch(ctx, msg)
			if err != nil {
				return allResults, fmt.Errorf("broadcast to subscriber %s: %w", sub.ID, err)
			}
			allResults = append(allResults, results...)
		} else {
			log.Printf("[GEO BROADCAST] Subscriber %s is %.2f miles away (Outside radius %.2f mi). Skipping.",
				sub.ID, distMiles, effectiveRadius)
		}
	}

	return allResults, nil
}
