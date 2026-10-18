package service

import (
	"context"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
)

type GeographyService struct {
	repo *repository.Repository
}

func NewGeographyService(repo *repository.Repository) *GeographyService {
	return &GeographyService{repo: repo}
}

type ZoneResponse struct {
	ID     string            `json:"id"`
	Code   string            `json:"code"`
	Name   string            `json:"name"`
	States []StateSummaryDTO `json:"states,omitempty"`
}

type StateSummaryDTO struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	IsFCT       bool   `json:"is_fct"`
	CapitalCity string `json:"capital_city,omitempty"`
}

type StateDetailResponse struct {
	ID          string   `json:"id"`
	ZoneID      string   `json:"zone_id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	IsFCT       bool     `json:"is_fct"`
	CapitalCity string   `json:"capital_city,omitempty"`
	LGAs        []db.Lga `json:"lgas,omitempty"`
}

func (s *GeographyService) ListZones(ctx context.Context) ([]ZoneResponse, error) {
	zones, err := s.repo.Queries().ListZones(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]ZoneResponse, len(zones))
	for i, z := range zones {
		states, _ := s.repo.Queries().ListStatesByZone(ctx, z.ID)
		stateDTOs := make([]StateSummaryDTO, len(states))
		for j, st := range states {
			capCity, _ := s.repo.Queries().GetCapitalCityByState(ctx, st.ID)
			stateDTOs[j] = StateSummaryDTO{
				ID:          st.ID,
				Code:        st.Code,
				Name:        st.Name,
				IsFCT:       st.IsFct,
				CapitalCity: capCity.Name,
			}
		}
		result[i] = ZoneResponse{
			ID:     z.ID,
			Code:   z.Code,
			Name:   z.Name,
			States: stateDTOs,
		}
	}
	return result, nil
}

func (s *GeographyService) ListStates(ctx context.Context) ([]StateDetailResponse, error) {
	states, err := s.repo.Queries().ListStates(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]StateDetailResponse, len(states))
	for i, st := range states {
		capCity, _ := s.repo.Queries().GetCapitalCityByState(ctx, st.ID)
		lgas, _ := s.repo.Queries().ListLGAsByState(ctx, st.ID)
		result[i] = StateDetailResponse{
			ID:          st.ID,
			ZoneID:      st.ZoneID,
			Code:        st.Code,
			Name:        st.Name,
			IsFCT:       st.IsFct,
			CapitalCity: capCity.Name,
			LGAs:        lgas,
		}
	}
	return result, nil
}

func (s *GeographyService) ListLGAsByState(ctx context.Context, stateID string) ([]db.Lga, error) {
	return s.repo.Queries().ListLGAsByState(ctx, stateID)
}
