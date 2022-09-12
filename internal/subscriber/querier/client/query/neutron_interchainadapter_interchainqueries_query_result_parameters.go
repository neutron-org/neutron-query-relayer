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

// NewNeutronInterchainadapterInterchainqueriesQueryResultParams creates a new NeutronInterchainadapterInterchainqueriesQueryResultParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewNeutronInterchainadapterInterchainqueriesQueryResultParams() *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	return &NeutronInterchainadapterInterchainqueriesQueryResultParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithTimeout creates a new NeutronInterchainadapterInterchainqueriesQueryResultParams object
// with the ability to set a timeout on a request.
func NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithTimeout(timeout time.Duration) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	return &NeutronInterchainadapterInterchainqueriesQueryResultParams{
		timeout: timeout,
	}
}

// NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithContext creates a new NeutronInterchainadapterInterchainqueriesQueryResultParams object
// with the ability to set a context for a request.
func NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithContext(ctx context.Context) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	return &NeutronInterchainadapterInterchainqueriesQueryResultParams{
		Context: ctx,
	}
}

// NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithHTTPClient creates a new NeutronInterchainadapterInterchainqueriesQueryResultParams object
// with the ability to set a custom HTTPClient for a request.
func NewNeutronInterchainadapterInterchainqueriesQueryResultParamsWithHTTPClient(client *http.Client) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	return &NeutronInterchainadapterInterchainqueriesQueryResultParams{
		HTTPClient: client,
	}
}

/* NeutronInterchainadapterInterchainqueriesQueryResultParams contains all the parameters to send to the API endpoint
   for the neutron interchainadapter interchainqueries query result operation.

   Typically these are written to a http.Request.
*/
type NeutronInterchainadapterInterchainqueriesQueryResultParams struct {

	// QueryID.
	//
	// Format: uint64
	QueryID *string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the neutron interchainadapter interchainqueries query result params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WithDefaults() *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the neutron interchainadapter interchainqueries query result params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WithTimeout(timeout time.Duration) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WithContext(ctx context.Context) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WithHTTPClient(client *http.Client) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithQueryID adds the queryID to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WithQueryID(queryID *string) *NeutronInterchainadapterInterchainqueriesQueryResultParams {
	o.SetQueryID(queryID)
	return o
}

// SetQueryID adds the queryId to the neutron interchainadapter interchainqueries query result params
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) SetQueryID(queryID *string) {
	o.QueryID = queryID
}

// WriteToRequest writes these params to a swagger request
func (o *NeutronInterchainadapterInterchainqueriesQueryResultParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

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