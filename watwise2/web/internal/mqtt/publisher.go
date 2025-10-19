package mqtt

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Publisher struct {
	client mqtt.Client
}

func NewPublisher(client mqtt.Client) *Publisher {
	return &Publisher{
		client: client,
	}
}

// PublishCommand publishes a command to device
func (p *Publisher) PublishCommand(deviceID string, command interface{}) error {
	topic := fmt.Sprintf("wattwise/commands/%s", deviceID)
	
	payload, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}
	
	token := p.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish command: %v", token.Error())
	}
	
	log.Printf("✅ Published command to device %s", deviceID)
	return nil
}

// PublishControlMessage publishes control message to device
func (p *Publisher) PublishControlMessage(deviceID, action string, params map[string]interface{}) error {
	topic := fmt.Sprintf("wattwise/control/%s", deviceID)
	
	message := map[string]interface{}{
		"action": action,
		"params": params,
	}
	
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal control message: %v", err)
	}
	
	token := p.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish control message: %v", token.Error())
	}
	
	log.Printf("✅ Published control message to device %s: %s", deviceID, action)
	return nil
}

// BroadcastMessage broadcasts message to all devices
func (p *Publisher) BroadcastMessage(message interface{}) error {
	topic := "wattwise/broadcast"
	
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %v", err)
	}
	
	token := p.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish broadcast: %v", err)
	}
	
	log.Println("✅ Broadcast message sent to all devices")
	return nil
}