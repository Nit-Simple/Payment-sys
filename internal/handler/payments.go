package handler

import "net/http"

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request)  {}
func (s *Server) handleListPayments(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleGetPayment(w http.ResponseWriter, r *http.Request)     {}
func (s *Server) handleConfirmPayment(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleCapturePayment(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleCancelPayment(w http.ResponseWriter, r *http.Request)  {}
func (s *Server) handleRefundPayment(w http.ResponseWriter, r *http.Request)  {}
