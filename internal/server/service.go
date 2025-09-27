package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	receiptspb "github.com/joseph-ayodele/receipts-tracker/gen/proto/receipts/v1"

	ent "github.com/joseph-ayodele/receipts-tracker/gen/ent"
	entprofile "github.com/joseph-ayodele/receipts-tracker/gen/ent/profile"
	entreceipt "github.com/joseph-ayodele/receipts-tracker/gen/ent/receipt"
)

type Service struct {
	receiptspb.UnimplementedReceiptsServiceServer
	ent *ent.Client
}

func New(entc *ent.Client) *Service {
	return &Service{ent: entc}
}

func (s *Service) CreateProfile(ctx context.Context, req *receiptspb.CreateProfileRequest) (*receiptspb.CreateProfileResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	cur := strings.TrimSpace(req.GetDefaultCurrency())
	if cur == "" {
		// Align with DB default 'USD'. If you prefer to let DB default apply instead,
		// we can remove this and set ent field optional — but this is simplest/explicit.
		cur = "USD"
	}
	// Basic normalization
	if len(cur) != 3 {
		return nil, status.Error(codes.InvalidArgument, "default_currency must be 3 letters (ISO 4217)")
	}
	cur = strings.ToUpper(cur)

	p, err := s.ent.Profile.
		Create().
		SetName(name).
		SetDefaultCurrency(cur).
		Save(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}

	return &receiptspb.CreateProfileResponse{
		Profile: toPBProfile(p),
	}, nil
}

func (s *Service) ListProfiles(ctx context.Context, _ *receiptspb.ListProfilesRequest) (*receiptspb.ListProfilesResponse, error) {
	plist, err := s.ent.Profile.
		Query().
		Order(entprofile.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profiles: %v", err)
	}
	out := make([]*receiptspb.Profile, 0, len(plist))
	for _, p := range plist {
		out = append(out, toPBProfile(p))
	}
	return &receiptspb.ListProfilesResponse{Profiles: out}, nil
}

func (s *Service) ListReceipts(ctx context.Context, req *receiptspb.ListReceiptsRequest) (*receiptspb.ListReceiptsResponse, error) {
	if strings.TrimSpace(req.GetProfileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id is required")
	}
	profileID, err := uuid.Parse(req.GetProfileId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "profile_id must be a UUID")
	}

	q := s.ent.Receipt.Query().Where(entreceipt.ProfileID(profileID))

	// Optional date range
	if fd := strings.TrimSpace(req.GetFromDate()); fd != "" {
		from, err := parseYMD(fd)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "from_date invalid (YYYY-MM-DD): %v", err)
		}
		q = q.Where(entreceipt.TxDateGTE(from))
	}
	if td := strings.TrimSpace(req.GetToDate()); td != "" {
		to, err := parseYMD(td)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "to_date invalid (YYYY-MM-DD): %v", err)
		}
		q = q.Where(entreceipt.TxDateLTE(to))
	}

	recs, err := q.
		Order(entreceipt.ByTxDate()).
		All(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list receipts: %v", err)
	}

	out := make([]*receiptspb.Receipt, 0, len(recs))
	for _, r := range recs {
		out = append(out, toPBReceipt(r))
	}
	return &receiptspb.ListReceiptsResponse{Receipts: out}, nil
}

func (s *Service) ExportReceipts(context.Context, *receiptspb.ExportReceiptsRequest) (*receiptspb.ExportReceiptsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExportReceipts not implemented yet (Step 8)")
}

func toPBProfile(p *ent.Profile) *receiptspb.Profile {
	return &receiptspb.Profile{
		Id:              p.ID.String(),
		Name:            p.Name,
		DefaultCurrency: p.DefaultCurrency,
		CreatedAt:       p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toPBReceipt(r *ent.Receipt) *receiptspb.Receipt {
	return &receiptspb.Receipt{
		Id:           r.ID.String(),
		ProfileId:    r.ProfileID.String(),
		MerchantName: r.MerchantName,
		TxDate:       r.TxDate.Format("2006-01-02"),
		Total:        fmt.Sprintf("%.2f", r.Total),
		CurrencyCode: r.CurrencyCode,
		CreatedAt:    r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func parseYMD(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	// strip time to midnight UTC to match DATE semantics
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}
