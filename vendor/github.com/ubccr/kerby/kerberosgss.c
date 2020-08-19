/**
 * Adopted from PyKerberos. Modified for use with Kerby.
 *
 * Copyright (c) 2006-2015 Apple Inc. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 **/

#include "kerberosgss.h"

#include "base64.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

gss_client_state* new_gss_client_state() {
    gss_client_state *state;

    state = (gss_client_state *) malloc(sizeof(gss_client_state));

    return state;
}

gss_server_state* new_gss_server_state() {
    gss_server_state *state;

    state = (gss_server_state *) malloc(sizeof(gss_server_state));

    return state;
}

void free_gss_client_state(gss_client_state *state) {
    free(state);
}

void free_gss_server_state(gss_server_state *state) {
    free(state);
}

int authenticate_gss_client_init(
    const char* service, const char* principal, long int gss_flags,
    gss_server_state* delegatestate, gss_client_state* state
)
{
    gss_buffer_desc name_token = GSS_C_EMPTY_BUFFER;
    gss_buffer_desc principal_token = GSS_C_EMPTY_BUFFER;
    int ret = AUTH_GSS_COMPLETE;
    
    state->server_name = GSS_C_NO_NAME;
    state->context = GSS_C_NO_CONTEXT;
    state->gss_flags = gss_flags;
    state->client_creds = GSS_C_NO_CREDENTIAL;
    state->username = NULL;
    state->response = NULL;
    
    // Import server name first
    name_token.length = strlen(service);
    name_token.value = (char *)service;
    
    state->maj_stat = gss_import_name(
        &state->min_stat, &name_token, gss_krb5_nt_service_name, &state->server_name
    );
    
    if (GSS_ERROR(state->maj_stat)) {
        ret = AUTH_GSS_ERROR;
        goto end;
    }
    // Use the delegate credentials if they exist
    if (delegatestate && delegatestate->client_creds != GSS_C_NO_CREDENTIAL) {
        state->client_creds = delegatestate->client_creds;
    }
    // If available use the principal to extract its associated credentials
    else if (principal && *principal) {
        gss_name_t name;
        principal_token.length = strlen(principal);
        principal_token.value = (char *)principal;

        state->maj_stat = gss_import_name(
            &state->min_stat, &principal_token, GSS_C_NT_USER_NAME, &name
        );
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
    	    goto end;
        }

        state->maj_stat = gss_acquire_cred(
            &state->min_stat, name, GSS_C_INDEFINITE, GSS_C_NO_OID_SET,
            GSS_C_INITIATE, &state->client_creds, NULL, NULL
        );
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }

        state->maj_stat = gss_release_name(&state->min_stat, &name);
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }
    }

end:
    return ret;
}

int authenticate_gss_client_clean(gss_client_state *state)
{
    OM_uint32 maj_stat;
    OM_uint32 min_stat;
    int ret = AUTH_GSS_COMPLETE;
    
    if (state->context != GSS_C_NO_CONTEXT) {
        maj_stat = gss_delete_sec_context(
            &min_stat, &state->context, GSS_C_NO_BUFFER
        );
    }
    if (state->server_name != GSS_C_NO_NAME) {
        maj_stat = gss_release_name(&min_stat, &state->server_name);
    }
    if (
        state->client_creds != GSS_C_NO_CREDENTIAL &&
        ! (state->gss_flags & GSS_C_DELEG_FLAG)
    ) {
        maj_stat = gss_release_cred(&min_stat, &state->client_creds);
    }
    if (state->username != NULL) {
        free(state->username);
        state->username = NULL;
    }
    if (state->response != NULL) {
        free(state->response);
        state->response = NULL;
    }
    
    return ret;
}

int authenticate_gss_client_step(
    gss_client_state* state, const char* challenge
) {
    gss_buffer_desc input_token = GSS_C_EMPTY_BUFFER;
    gss_buffer_desc output_token = GSS_C_EMPTY_BUFFER;
    int ret = AUTH_GSS_CONTINUE;
    
    // Always clear out the old response
    if (state->response != NULL) {
        free(state->response);
        state->response = NULL;
    }
    
    // If there is a challenge (data from the server) we need to give it to GSS
    if (challenge && *challenge) {
        size_t len;
        input_token.value = base64_decode(challenge, &len);
        input_token.length = len;
    }
    
    // Do GSSAPI step
    state->maj_stat = gss_init_sec_context(
        &state->min_stat,
        state->client_creds,
        &state->context,
        state->server_name,
        GSS_C_NO_OID,
        (OM_uint32)state->gss_flags,
        0,
        GSS_C_NO_CHANNEL_BINDINGS,
        &input_token,
        NULL,
        &output_token,
        NULL,
        NULL
    );
    
    if ((state->maj_stat != GSS_S_COMPLETE) && (state->maj_stat != GSS_S_CONTINUE_NEEDED)) {
        ret = AUTH_GSS_ERROR;
        goto end;
    }
    
    ret = (state->maj_stat == GSS_S_COMPLETE) ? AUTH_GSS_COMPLETE : AUTH_GSS_CONTINUE;
    // Grab the client response to send back to the server
    if (output_token.length) {
        state->response = base64_encode((const unsigned char *)output_token.value, output_token.length);;
        state->maj_stat = gss_release_buffer(&state->min_stat, &output_token);
    }
    
    // Try to get the user name if we have completed all GSS operations
    if (ret == AUTH_GSS_COMPLETE) {
        gss_name_t gssuser = GSS_C_NO_NAME;
        state->maj_stat = gss_inquire_context(&state->min_stat, state->context, &gssuser, NULL, NULL, NULL,  NULL, NULL, NULL);
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }
        
        gss_buffer_desc name_token;
        name_token.length = 0;
        state->maj_stat = gss_display_name(&state->min_stat, gssuser, &name_token, NULL);
        if (GSS_ERROR(state->maj_stat)) {
            if (name_token.value)
                gss_release_buffer(&state->min_stat, &name_token);
            gss_release_name(&state->min_stat, &gssuser);
            
            ret = AUTH_GSS_ERROR;
            goto end;
        } else {
            state->username = (char *)malloc(name_token.length + 1);
            strncpy(state->username, (char*) name_token.value, name_token.length);
            state->username[name_token.length] = 0;
            gss_release_buffer(&state->min_stat, &name_token);
            gss_release_name(&state->min_stat, &gssuser);
        }
    }

end:
    if (output_token.value) {
        gss_release_buffer(&state->min_stat, &output_token);
    }
    if (input_token.value) {
        free(input_token.value);
    }
    return ret;
}

int authenticate_gss_server_init(const char *service, gss_server_state *state)
{
    gss_buffer_desc name_token = GSS_C_EMPTY_BUFFER;
    int ret = AUTH_GSS_COMPLETE;
    
    state->context = GSS_C_NO_CONTEXT;
    state->server_name = GSS_C_NO_NAME;
    state->client_name = GSS_C_NO_NAME;
    state->server_creds = GSS_C_NO_CREDENTIAL;
    state->client_creds = GSS_C_NO_CREDENTIAL;
    state->username = NULL;
    state->targetname = NULL;
    state->response = NULL;
    state->ccname = NULL;
    
    // Server name may be empty which means we aren't going to create our own creds
    size_t service_len = strlen(service);
    if (service_len != 0) {
        // Import server name first
        name_token.length = strlen(service);
        name_token.value = (char *)service;
        
        state->maj_stat = gss_import_name(
            &state->min_stat, &name_token, GSS_C_NT_HOSTBASED_SERVICE,
            &state->server_name
        );
        
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }

        // Get credentials
        state->maj_stat = gss_acquire_cred(
            &state->min_stat, GSS_C_NO_NAME, GSS_C_INDEFINITE, GSS_C_NO_OID_SET,
            GSS_C_BOTH, &state->server_creds, NULL, NULL
        );

        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }
    }
    
end:
    return ret;
}

int authenticate_gss_server_clean(gss_server_state *state)
{
    int ret = AUTH_GSS_COMPLETE;
    
    if (state->context != GSS_C_NO_CONTEXT) {
        state->maj_stat = gss_delete_sec_context(
            &state->min_stat, &state->context, GSS_C_NO_BUFFER
        );
    }
    if (state->server_name != GSS_C_NO_NAME) {
        state->maj_stat = gss_release_name(&state->min_stat, &state->server_name);
    }
    if (state->client_name != GSS_C_NO_NAME) {
        state->maj_stat = gss_release_name(&state->min_stat, &state->client_name);
    }
    if (state->server_creds != GSS_C_NO_CREDENTIAL) {
        state->maj_stat = gss_release_cred(&state->min_stat, &state->server_creds);
    }
    if (state->client_creds != GSS_C_NO_CREDENTIAL) {
        state->maj_stat = gss_release_cred(&state->min_stat, &state->client_creds);
    }
    if (state->username != NULL) {
        free(state->username);
        state->username = NULL;
    }
    if (state->targetname != NULL) {
        free(state->targetname);
        state->targetname = NULL;
    }
    if (state->response != NULL) {
        free(state->response);
        state->response = NULL;
    }
    if (state->ccname != NULL) {
        free(state->ccname);
        state->ccname = NULL;
    }
    
    return ret;
}

int authenticate_gss_server_step(
    gss_server_state *state, const char *challenge
) {
    gss_buffer_desc input_token = GSS_C_EMPTY_BUFFER;
    gss_buffer_desc output_token = GSS_C_EMPTY_BUFFER;
    int ret = AUTH_GSS_CONTINUE;
    
    // Always clear out the old response
    if (state->response != NULL) {
        free(state->response);
        state->response = NULL;
    }
    
    // If there is a challenge (data from the server) we need to give it to GSS
    if (challenge && *challenge) {
        size_t len;
        input_token.value = base64_decode(challenge, &len);
        input_token.length = len;
    } else {
        // XXX No challenge parameter in request from client
        // XXX How to pass error string to state? 
        ret = AUTH_GSS_ERROR;
        goto end;
    }
    
    state->maj_stat = gss_accept_sec_context(
        &state->min_stat,
        &state->context,
        state->server_creds,
        &input_token,
        GSS_C_NO_CHANNEL_BINDINGS,
        &state->client_name,
        NULL,
        &output_token,
        NULL,
        NULL,
        &state->client_creds
    );
    
    if (GSS_ERROR(state->maj_stat)) {
        ret = AUTH_GSS_ERROR;
        goto end;
    }
    
    // Grab the server response to send back to the client
    if (output_token.length) {
        state->response = base64_encode(
            (const unsigned char *)output_token.value, output_token.length
        );;
        state->maj_stat = gss_release_buffer(&state->min_stat, &output_token);
    }
    
    // Get the user name
    state->maj_stat = gss_display_name(
        &state->min_stat, state->client_name, &output_token, NULL
    );
    if (GSS_ERROR(state->maj_stat)) {
        ret = AUTH_GSS_ERROR;
        goto end;
    }
    state->username = (char *)malloc(output_token.length + 1);
    strncpy(state->username, (char*) output_token.value, output_token.length);
    state->username[output_token.length] = 0;
    
    // Get the target name if no server creds were supplied
    if (state->server_creds == GSS_C_NO_CREDENTIAL) {
        gss_name_t target_name = GSS_C_NO_NAME;
        state->maj_stat = gss_inquire_context(
            &state->min_stat, state->context, NULL, &target_name, NULL, NULL, NULL,
            NULL, NULL
        );
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }
        state->maj_stat = gss_display_name(
            &state->min_stat, target_name, &output_token, NULL
        );
        if (GSS_ERROR(state->maj_stat)) {
            ret = AUTH_GSS_ERROR;
            goto end;
        }
        state->targetname = (char *)malloc(output_token.length + 1);
        strncpy(
            state->targetname, (char*) output_token.value, output_token.length
        );
        state->targetname[output_token.length] = 0;
    }

    ret = AUTH_GSS_COMPLETE;
    
end:
    if (output_token.length) {
        gss_release_buffer(&state->min_stat, &output_token);
    }
    if (input_token.value) {
        free(input_token.value);
    }
    return ret;
}

void get_gss_error(OM_uint32 err_maj, char *buf_maj, OM_uint32 err_min, char *buf_min)
{
    OM_uint32 maj_stat, min_stat;
    OM_uint32 msg_ctx = 0;
    gss_buffer_desc status_string;
    
    do {
        maj_stat = gss_display_status(
            &min_stat,
            err_maj,
            GSS_C_GSS_CODE,
            GSS_C_NO_OID,
            &msg_ctx,
            &status_string
        );
        if (GSS_ERROR(maj_stat)) {
            break;
        }
        strncpy(buf_maj, (char*) status_string.value, GSS_ERRBUF_SIZE);
        gss_release_buffer(&min_stat, &status_string);
        
        maj_stat = gss_display_status(
            &min_stat,
            err_min,
            GSS_C_MECH_CODE,
            GSS_C_NULL_OID,
            &msg_ctx,
            &status_string
        );
        if (! GSS_ERROR(maj_stat)) {
            strncpy(buf_min, (char*) status_string.value, GSS_ERRBUF_SIZE);
            gss_release_buffer(&min_stat, &status_string);
        }
    } while (!GSS_ERROR(maj_stat) && msg_ctx != 0);
}

