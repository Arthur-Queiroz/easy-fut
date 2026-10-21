package store

import "time"

type User struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Phone          string     `json:"phone"`
	AvatarColor    string     `json:"avatar_color"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	Kind           string     `json:"kind"` // "member" ou "guest" (avulso, sem conta)
	InactiveReason string     `json:"inactive_reason"`
	Delinquent     bool       `json:"delinquent"`
	LastPaymentAt  *time.Time `json:"last_payment_at,omitempty"`
	SessionVersion int        `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Invite struct {
	ID          string     `json:"id"`
	Token       string     `json:"token"`
	InvitedName string     `json:"invited_name"`
	Phone       string     `json:"phone"`
	Role        string     `json:"role"`
	CreatedBy   string     `json:"created_by"`
	CreatorName string     `json:"creator_name"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at,omitempty"`
	UsedBy      *string    `json:"used_by,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	AccessCount int        `json:"access_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Match struct {
	ID                   string     `json:"id"`
	MatchDate            string     `json:"match_date"` // YYYY-MM-DD
	StartTime            string     `json:"start_time"` // HH:MM
	EndTime              string     `json:"end_time"`
	Venue                string     `json:"venue"`
	Address              string     `json:"address"`
	ConfirmationDeadline time.Time  `json:"confirmation_deadline"`
	Status               string     `json:"status"`
	CancelReason         string     `json:"cancel_reason"`
	Notes                string     `json:"notes"`
	VotingClosesAt       *time.Time `json:"voting_closes_at,omitempty"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`

	GoingCount      int `json:"going_count"`
	NotGoingCount   int `json:"not_going_count"`
	NoResponseCount int `json:"no_response_count"`
	MediaCount      int `json:"media_count"`

	// Venda de vagas avulsas: preço e total congelados ao abrir a venda.
	GuestSaleOpen       bool  `json:"guest_sale_open"`
	GuestSlots          int   `json:"guest_slots"`
	GuestPriceCents     int64 `json:"guest_price_cents"`
	GuestTotalCents     int64 `json:"guest_total_cents"`
	GuestConfirmedCount int   `json:"guest_confirmed_count"` // inscrições confirmed
	GuestPendingCount   int   `json:"guest_pending_count"`   // reserved + pending_payment
}

type ConfirmationEntry struct {
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	AvatarColor string     `json:"avatar_color"`
	Role        string     `json:"role"`
	Response    string     `json:"response"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
}

type Team struct {
	ID        string       `json:"id"`
	MatchID   string       `json:"match_id"`
	TeamName  string       `json:"team_name"`
	TeamColor string       `json:"team_color"`
	Position  int          `json:"position"`
	Members   []TeamMember `json:"members"`
}

type TeamMember struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	AvatarColor string `json:"avatar_color"`
}

// TeamPreset é um grupo salvo de jogadores para agilizar a escalação manual.
type TeamPreset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MemberIDs []string  `json:"member_ids"`
	CreatedAt time.Time `json:"created_at"`
}

type ChargeBatch struct {
	ID                    string    `json:"id"`
	ReferenceMonth        string    `json:"reference_month"` // YYYY-MM
	Kind                  string    `json:"kind"`            // "monthly" ou "match"
	Title                 string    `json:"title"`           // rótulo dos lotes de partida avulsa
	TotalAmountCents      int64     `json:"total_amount_cents"`
	UserCount             int       `json:"user_count"`
	IndividualAmountCents int64     `json:"individual_amount_cents"`
	DueDate               string    `json:"due_date"`      // YYYY-MM-DD
	PixFeeBasisPoints     int64     `json:"pix_fee_bps"`   // taxa do Pix congelada no lote
	PixFeeCents           int64     `json:"pix_fee_cents"` // taxa por pessoa no Pix
	GeneratedBy           string    `json:"generated_by"`
	GeneratedByName       string    `json:"generated_by_name"`
	CreatedAt             time.Time `json:"created_at"`
}

type Charge struct {
	ID              string     `json:"id"`
	BatchID         string     `json:"batch_id"`
	BatchKind       string     `json:"batch_kind"`
	BatchTitle      string     `json:"batch_title"`
	UserID          string     `json:"user_id"`
	UserName        string     `json:"user_name"`
	UserRole        string     `json:"user_role"`
	AvatarColor     string     `json:"avatar_color"`
	ReferenceMonth  string     `json:"reference_month"` // YYYY-MM
	AmountCents     int64      `json:"amount_cents"`    // rateio
	PixFeeCents     int64      `json:"pix_fee_cents"`   // taxa do Pix repassada
	PixTotalCents   int64      `json:"pix_total_cents"` // o que o jogador paga no Pix
	Status          string     `json:"status"`
	DueDate         string     `json:"due_date"` // YYYY-MM-DD
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	PaidMethod      string     `json:"paid_method"`
	RegisteredBy    *string    `json:"registered_by,omitempty"`
	RegisteredName  string     `json:"registered_by_name,omitempty"`
	PixPayload      string     `json:"pix_payload"`
	PixTicketURL    string     `json:"pix_ticket_url"`
	PixQRCodeBase64 string     `json:"pix_qr_code_base64"`
	ProviderOrderID string     `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
}

type VoteResult struct {
	Category  string   `json:"category"`
	VoteCount int      `json:"vote_count"`
	Winners   []Winner `json:"winners"`
}

type Winner struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	AvatarColor string `json:"avatar_color"`
	Votes       int    `json:"votes"`
}

type Media struct {
	ID           string    `json:"id"`
	MatchID      string    `json:"match_id"`
	UploadedBy   string    `json:"uploaded_by"`
	UploaderName string    `json:"uploader_name"`
	Type         string    `json:"type"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Caption      string    `json:"caption"`
	CreatedAt    time.Time `json:"created_at"`
}

type Activity struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// GuestBooking é a inscrição de um avulso numa partida. Valores congelados na
// compra: base (vai para o caixa), taxa do Pix embutida e total cobrado.
type GuestBooking struct {
	ID                   string     `json:"id"`
	MatchID              string     `json:"match_id"`
	UserID               string     `json:"user_id"`
	Name                 string     `json:"name"`
	Phone                string     `json:"phone"` // E.164
	Email                string     `json:"email"`
	Status               string     `json:"status"`
	BaseCents            int64      `json:"base_cents"`
	TotalCents           int64      `json:"total_cents"`
	FeeCents             int64      `json:"fee_cents"`
	ExpiresAt            time.Time  `json:"expires_at"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
	AccessTokenHash      string     `json:"-"`
	ProviderOrderID      string     `json:"-"`
	ProviderPaymentID    string     `json:"-"`
	PixPayload           string     `json:"pix_payload"`
	PixQRCodeBase64      string     `json:"pix_qr_code_base64"`
	PixTicketURL         string     `json:"pix_ticket_url"`
	ProviderStatus       string     `json:"-"`
	ProviderStatusDetail string     `json:"-"`
	RefundReason         string     `json:"refund_reason"`
	CreatedAt            time.Time  `json:"created_at"`
}

// CashEntry é um lançamento do caixa do grupo. O valor é positivo; o sinal
// vem do tipo (guests.CashSign).
type CashEntry struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"` // guest_payment, refund, expense
	AmountCents   int64     `json:"amount_cents"`
	FeeCents      int64     `json:"fee_cents"`
	BookingID     *string   `json:"booking_id"` // nulo em despesa
	Description   string    `json:"description"`
	OccurredOn    string    `json:"occurred_on"` // YYYY-MM-DD
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// CashReport é o caixa: o saldo de todos os lançamentos e a lista do período.
type CashReport struct {
	BalanceCents int64       `json:"balance_cents"`
	Entries      []CashEntry `json:"entries"`
}
