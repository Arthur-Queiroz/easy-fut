// Tipos espelhando o JSON da API Go.

export interface User {
  id: string
  name: string
  email: string
  phone: string
  avatar_color: string
  role: 'admin' | 'player'
  status: 'active' | 'inactive' | 'archived'
  inactive_reason: string
  delinquent: boolean
  last_payment_at?: string
  created_at: string
}

export interface UserStats {
  matches_played: number
  top_scorer_count: number
  worst_count: number
}

export interface Invite {
  id: string
  token: string
  invited_name: string
  role: 'admin' | 'player'
  creator_name: string
  expires_at: string
  used_at?: string
  revoked_at?: string
  access_count: number
  created_at: string
}

export type InvitePublicStatus = 'pending' | 'used' | 'revoked' | 'expired'

export interface InvitePublic {
  invited_name: string
  creator_name: string
  expires_at: string
  status: InvitePublicStatus
}

export type MatchStatus = 'open' | 'closed' | 'teams_drawn' | 'voting' | 'finished' | 'cancelled'

export interface Match {
  id: string
  match_date: string
  start_time: string
  end_time: string
  venue: string
  address: string
  confirmation_deadline: string
  status: MatchStatus
  cancel_reason: string
  notes: string
  voting_closes_at?: string
  finished_at?: string
  going_count: number
  not_going_count: number
  no_response_count: number
  media_count: number
  results?: VoteResult[]
}

export type Response = 'going' | 'not_going' | 'no_response'

export interface ConfirmationEntry {
  user_id: string
  name: string
  avatar_color: string
  role: 'admin' | 'player'
  response: Response
  responded_at?: string
}

export interface Team {
  id: string
  match_id: string
  team_name: string
  team_color: string
  position: number
  members: TeamMember[]
}

export interface TeamMember {
  user_id: string
  name: string
  avatar_color: string
}

export type ChargeStatus = 'pending' | 'paid' | 'manual_paid' | 'overdue' | 'cancelled' | 'exempt'

export interface Charge {
  id: string
  user_id: string
  user_name: string
  user_role: 'admin' | 'player'
  avatar_color: string
  reference_month: string
  amount_cents: number
  status: ChargeStatus
  due_date: string
  paid_at?: string
  paid_method: string
  registered_by_name?: string
  pix_payload: string
  pix_ticket_url: string
  pix_qr_code_base64: string
}

export interface ChargeBatch {
  id: string
  reference_month: string
  total_amount_cents: number
  user_count: number
  individual_amount_cents: number
  due_date: string
  generated_by_name: string
  created_at: string
}

export interface VoteResult {
  category: 'top_scorer' | 'worst_player'
  vote_count: number
  winners: Winner[]
}

export interface Winner {
  user_id: string
  name: string
  avatar_color: string
  votes: number
}

export interface Media {
  id: string
  match_id: string
  uploaded_by: string
  uploader_name: string
  type: 'photo' | 'video'
  url: string
  caption: string
  created_at: string
}

export interface Activity {
  id: string
  kind: string
  message: string
  created_at: string
}

export interface Dashboard {
  stats: {
    active_users: number
    inactive_users: number
    delinquents: number
    pending_invites: number
    month_paid_cents: number
    month_due_cents: number
    paid_count: number
    pending_count: number
    charge_count: number
  }
  delinquents: Charge[]
  activity: Activity[]
  next_match: Match | null
}

export interface MatchDetail {
  match: Match
  confirmations: ConfirmationEntry[]
  teams: Team[]
  media: Media[]
  is_participant: boolean
  my_votes: Record<string, string>
  results: VoteResult[] | null
}

export interface NextMatch {
  match: Match | null
  my_response?: Response
  confirmations?: ConfirmationEntry[]
}
