// Code generated by go-swagger; DO NOT EDIT.

package query

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewNeutronInterchainQueriesRegisteredQueryParams creates a new NeutronInterchainQueriesRegisteredQueryParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewNeutronInterchainQueriesRegisteredQueryParams() *NeutronInterchainQueriesRegisteredQueryParams {
	return &NeutronInterchainQueriesRegisteredQueryParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewNeutronInterchainQueriesRegisteredQueryParamsWithTimeout creates a new NeutronInterchainQueriesRegisteredQueryParams object
// with the ability to set a timeout on a request.
func NewNeutronInterchainQueriesRegisteredQueryParamsWithTimeout(timeout time.Duration) *NeutronInterchainQueriesRegisteredQueryParams {
	return &NeutronInterchainQueriesRegisteredQueryParams{
		timeout: timeout,
	}
}

// NewNeutronInterchainQueriesRegisteredQueryParamsWithContext creates a new NeutronInterchainQueriesRegisteredQueryParams object
// with the ability to set a context for a request.
func NewNeutronInterchainQueriesRegisteredQueryParamsWithContext(ctx context.Context) *NeutronInterchainQueriesRegisteredQueryParams {
	return &NeutronInterchainQueriesRegisteredQueryParams{
		Context: ctx,
	}
}

// NewNeutronInterchainQueriesRegisteredQueryParamsWithHTTPClient creates a new NeutronInterchainQueriesRegisteredQueryParams object
// with the ability to set a custom HTTPClient for a request.
func NewNeutronInterchainQueriesRegisteredQueryParamsWithHTTPClient(client *http.Client) *NeutronInterchainQueriesRegisteredQueryParams {
	return &NeutronInterchainQueriesRegisteredQueryParams{
		HTTPClient: client,
	}
}

/*
NeutronInterchainQueriesRegisteredQueryParams contains all the parameters to send to the API endpoint

	for the neutron interchain queries registered query operation.

	Typically these are written to a http.Request.
*/
type NeutronInterchainQueriesRegisteredQueryParams struct {

	// QueryID.
	//
	// Format: uint64
	QueryID *string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the neutron interchain queries registered query params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *NeutronInterchainQueriesRegisteredQueryParams) WithDefaults() *NeutronInterchainQueriesRegisteredQueryParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the neutron interchain queries registered query params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *NeutronInterchainQueriesRegisteredQueryParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) WithTimeout(timeout time.Duration) *NeutronInterchainQueriesRegisteredQueryParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) WithContext(ctx context.Context) *NeutronInterchainQueriesRegisteredQueryParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) WithHTTPClient(client *http.Client) *NeutronInterchainQueriesRegisteredQueryParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithQueryID adds the queryID to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) WithQueryID(queryID *string) *NeutronInterchainQueriesRegisteredQueryParams {
	o.SetQueryID(queryID)
	return o
}

// SetQueryID adds the queryId to the neutron interchain queries registered query params
func (o *NeutronInterchainQueriesRegisteredQueryParams) SetQueryID(queryID *string) {
	o.QueryID = queryID
}

// WriteToRequest writes these params to a swagger request
func (o *NeutronInterchainQueriesRegisteredQueryParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.QueryID != nil {

		// query param query_id
		var qrQueryID string

		if o.QueryID != nil {
			qrQueryID = *o.QueryID
		}
		qQueryID := qrQueryID
		if qQueryID != "" {

			if err := r.SetQueryParam("query_id", qQueryID); err != nil {
				return err
			}
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}