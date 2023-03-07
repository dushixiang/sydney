package sydney

import "time"

type Conversation struct {
	ConversationId        string `json:"conversationId"`
	ClientId              string `json:"clientId"`
	ConversationSignature string `json:"conversationSignature"`
	Result                struct {
		Value   string `json:"value"`
		Message string `json:"message"`
	} `json:"result"`
}

type Question struct {
	Arguments []struct {
		Source              string        `json:"source"`
		OptionsSets         []string      `json:"optionsSets"`
		AllowedMessageTypes []string      `json:"allowedMessageTypes"`
		SliceIds            []interface{} `json:"sliceIds"`
		TraceId             string        `json:"traceId"`
		IsStartOfSession    bool          `json:"isStartOfSession"`
		Message             struct {
			Locale        string `json:"locale"`
			Market        string `json:"market"`
			Region        string `json:"region"`
			Location      string `json:"location"`
			LocationHints []struct {
				Country           string `json:"country"`
				State             string `json:"state"`
				City              string `json:"city"`
				Timezoneoffset    int    `json:"timezoneoffset"`
				CountryConfidence int    `json:"countryConfidence"`
				CityConfidence    int    `json:"cityConfidence"`
				Center            struct {
					Latitude  float64 `json:"Latitude"`
					Longitude float64 `json:"Longitude"`
				} `json:"Center"`
				RegionType int `json:"RegionType"`
				SourceType int `json:"SourceType"`
			} `json:"locationHints"`
			Timestamp   time.Time `json:"timestamp"`
			Author      string    `json:"author"`
			InputMethod string    `json:"inputMethod"`
			Text        string    `json:"text"`
			MessageType string    `json:"messageType"`
		} `json:"message"`
		ConversationSignature string `json:"conversationSignature"`
		Participant           struct {
			Id string `json:"id"`
		} `json:"participant"`
		ConversationId string `json:"conversationId"`
	} `json:"arguments"`
	InvocationId string `json:"invocationId"`
	Target       string `json:"target"`
	Type         int    `json:"type"`
}
