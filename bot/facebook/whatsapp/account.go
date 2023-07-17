package whatsapp

// https://developers.facebook.com/docs/graph-api/reference/whats-app-business-account-to-number-current-status/#fields
type WhatsAppBusinessAccountToNumberCurrentStatus struct {
	// The WABA Phone Number ID.
	ID string `json:"id,omitempty"`

	// International format representation of the phone number
	PhoneNumber string `json:"display_phone_number,omitempty"`

	// The name registered in this phone's business profile
	// that will be displayed to customers.
	//
	// Note: This is slightly misnamed now
	// that name that have yet to be verified can be registered
	VerifiedName string `json:"verified_name,omitempty"`

	// enum
	// The account mode of the phone number
	AccountMode string `json:"account_mode,omitempty"`

	// Indicates if phone number is associated with an Official Business Account.
	IsOfficial bool `json:"is_official_business_account,omitempty"`

	// Certificate of the phone number.
	Certificate string `json:"certificate,omitempty"`

	// enum
	// Indicates the phone number's one-time password (OTP) verification status.
	// Values can be NOT_VERIFIED or VERIFIED.
	// Only phone numbers with a VERIFIED status can be registered.
	//
	// Note that once OTP verification has been completed,
	// the phone number's status will be VERIFIED for 14 days,
	// after which OTP must be completed again before the phone number can be registered.
	//
	// See Register Phone Numbers and Get Phone's OTP Status.
	//
	// Default
	CodeVerificationStatus string `json:"code_verification_status,omitempty"`

	// Status of eligibility in the API Business Global Search.
	Eligibility string `json:"eligibility_for_api_business_global_search,omitempty"`

	// Returns True if a pin for two-step verification is enabled.
	IsPinEnabled bool `json:"is_pin_enabled,omitempty"`

	// enum {TIER_50, TIER_250, TIER_1K, TIER_10K, TIER_100K, TIER_UNLIMITED}
	// Current messaging limit tier
	MessagingLimitTier string `json:"messaging_limit_tier,omitempty"`

	// enum
	// The status of the name review
	// https://developers.facebook.com/docs/whatsapp/business-management-api/manage-phone-numbers#get-display-name-status--beta-
	NameStatus string `json:"name_status,omitempty"`

	// Certificate of the new name that was requested
	NewCertificate string `json:"new_certificate,omitempty"`

	// enum
	// The status of the review of the new name requested
	NewNameStatus string `json:"new_name_status,omitempty"`

	// WhatsAppBusinessAccountToNumberCurrentStatusWhatsAppBusinessPhoneQualityScoreShape
	// Quality score of the phone
	QualityScore interface{} `json:"quality_score,omitempty"`

	// The availability of the phone_number in the WhatsApp Business search.
	SearchVisibility string `json:"search_visibility,omitempty"`

	// enum {PENDING, DELETED, MIGRATED, BANNED, RESTRICTED, RATE_LIMITED, FLAGGED, CONNECTED, DISCONNECTED, UNKNOWN, UNVERIFIED}
	// The operating status of the phone number (eg. connected, rate limited, warned)
	Status string `json:"status,omitempty"`
}

// https://developers.facebook.com/docs/graph-api/reference/whats-app-business-account#fields
type WhatsAppBusinessAccount struct {

	// ID of the WhatApp Business Account.
	ID string `json:"id,omitempty"`

	// User-friendly name to differentiate WhatsApp Business Accounts
	Name string `json:"name,omitempty"`

	// Country of the WhatsApp Business Account's owning Meta Business account
	Country string `json:"country,omitempty"`

	// The currency in which the payment transactions for the WhatsApp Business Account will be processed
	Currency string `json:"currency,omitempty"`

	// The timezone of the WhatsApp Business Account
	TimezoneID string `json:"timezone_id,omitempty"`

	// WABAAnalytics
	// Analytics data of the WhatsApp Business Account.
	Analytics interface{} `json:"analytics,omitempty"`

	// enum
	// Status from account review process.
	ReviewStatus string `json:"account_review_status,omitempty"`

	// enum
	// Ownership type of the WhatsApp Business Account
	OwnershipType string `json:"ownership_type,omitempty"`

	// Primary funding ID for the WhatsApp Business Account paid service
	PrimaryFundingID string `json:"primary_funding_id,omitempty"`

	// The purchase order number supplied by the business for payment management purposes
	PurchaseOrderNumber string `json:"purchase_order_number,omitempty"`

	// Namespace string for the message templates that belong to the WhatsApp Business Account
	MessageTemplateNamespace string `json:"message_template_namespace,omitempty"`

	// WABAOnBehalfOfComputedInfo
	// The "on behalf of" information for the WhatsApp Business Account
	OnBehalfOfBusinessInfo interface{} `json:"on_behalf_of_business_info,omitempty"`

	// enum
	// Current status of business verification of Meta Business Account which owns this WhatsApp Business Account
	BusinessVerificationStatus string `json:"business_verification_status,omitempty"`

	// ----------------- Edges ----------------- //

	// // Analytics data of the WhatsApp Business Account with conversation based pricing
	// ConversationAnalytics interface{} `json:"conversation_analytics,omitempty"`

	// // Message templates that belong to the WhatsApp Business Account
	// MessageTemplates interface{} `json:"message_templates,omitempty"`

	// The phone numbers that belong to the WhatsApp Business Account
	PhoneNumbers []*WhatsAppBusinessAccountToNumberCurrentStatus `json:"phone_numbers,omitempty"`

	// // product_catalogs
	// ProductCatalogs interface{} `json:"product_catalogs,omitempty"`

	// // List of apps that are subscribed to webhooks updates for this WABA
	// SubscribedApps interface{} `json:"subscribed_apps,omitempty"`

	// EXTENSION: Internal USE only !
	// NOTE: Since `subscribed_apps` returns the only
	// our client's authorized app if subscribed,
	// we`ve decided to display `subscribed_fields` instead.
	//
	// None-empty set of fields subscribed on Webhooks WhatsAppBusinessAccount object.
	// Indicates whether this WHatsApp Business Account is subscribed at all.
	SubscribedFields []string `json:"subscribed_fields,omitempty"`
}

// IndexPhoneNumberID returns index of
// .PhoneNumbers[ID|PhoneNumber] that MATCH given WAID.
// Returns -1 if NOT FOUND.
func (ba *WhatsAppBusinessAccount) IndexPhoneNumber(WAID string) int {
	// Precondition(s)
	if WAID == "" || ba == nil || len(ba.PhoneNumbers) == 0 {
		return -1 // NOT FOUND
	}
	var (
		i  int // Zero(0)
		wa *WhatsAppBusinessAccountToNumberCurrentStatus
	)
	for ; i < len(ba.PhoneNumbers); i++ {
		wa = ba.PhoneNumbers[i]
		if wa.ID == WAID {
			// MATCH: NUMBER_ID
			break
		}
		if wa.PhoneNumber == WAID {
			// MATCH: PHONE_NUMBER
			break
		}
	}
	if i == len(ba.PhoneNumbers) {
		// NOT FOUND
		return -1
	}
	return i
}

func (ba *WhatsAppBusinessAccount) GetPhoneNumber(WAID string) *WhatsAppBusinessAccountToNumberCurrentStatus {
	if e := ba.IndexPhoneNumber(WAID); e != -1 {
		return ba.PhoneNumbers[e]
	}
	return nil
}

// AddPhoneNumber register WhatsApp Business unique PhoneNumber `account`.
// `reset` indicates whether UPDATE existed `account` WITH given ONE ?
func (ba *WhatsAppBusinessAccount) AddPhoneNumber(account *WhatsAppBusinessAccountToNumberCurrentStatus, reset bool) (created bool) {
	WAID := account.ID
	if e := ba.IndexPhoneNumber(WAID); e == -1 { // CREATE
		ba.PhoneNumbers = append(ba.PhoneNumbers, account)
		created = true
	} else if reset { // EXISTS
		ba.PhoneNumbers[e] = account
	}
	return // created
}

// DelPhoneNumber deregister WhatsApp Business unique PhoneNumber `account`.
// `exact` indicates whether DELETE existed `account` if given ONE is a clone ?
func (ba *WhatsAppBusinessAccount) DelPhoneNumber(account *WhatsAppBusinessAccountToNumberCurrentStatus, exact bool) (removed *WhatsAppBusinessAccountToNumberCurrentStatus) {
	WAID := account.ID
	if e := ba.IndexPhoneNumber(WAID); e != -1 { // EXISTS
		removed = ba.PhoneNumbers[e]
		if exact && removed != account {
			removed = nil
			return // ABORT
		}
		ba.PhoneNumbers = append(
			ba.PhoneNumbers[0:e],
			ba.PhoneNumbers[e+1:]...,
		)
	}
	return // removed
}
