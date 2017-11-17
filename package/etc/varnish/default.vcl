vcl 4.0;
backend default {
        .host = "127.0.0.1";
        .port = "8080";
        .probe = {
                .url = "/";
                .timeout = 5000 ms;
                .interval = 5s;
                .window = 2;
                .threshold = 2;
        }
}

//backend stalk {
//        .host = "127.0.0.1";
//        .port = "5000";
//        .probe = {
//                .url = "/";
//                .timeout = 5000 ms;
//                .interval = 5s;
//                .window = 2;
//                .threshold = 2;
//        }
//}

sub vcl_recv {
//        if ( req.http.host ~ "^stalk" ) {
//               set req.backend_hint = stalk;
//        }

        // Handle compression correctly. Different browsers send different
        // "Accept-Encoding" headers, even though they mostly all support the same
        // compression mechanisms. By consolidating these compression headers into
        // a consistent format, we can reduce the size of the cache and get more hits.
        // @see: http://varnish.projects.linpro.no/wiki/FAQ/Compression
        if ( req.http.Accept-Encoding ) {
                if ( req.http.Accept-Encoding ~ "gzip" ) {
                        # If the browser supports it, we'll use gzip.
                        set req.http.Accept-Encoding = "gzip";
                }
                else if ( req.http.Accept-Encoding ~ "deflate" ) {
                        # Next, try deflate if it is supported.
                        set req.http.Accept-Encoding = "deflate";
                }
                else {
                        # Unknown algorithm. Remove it and send unencoded.
                        unset req.http.Accept-Encoding;
                }
        }
        unset req.http.cookie;
}

sub vcl_backend_response {
        // cache static content - cloudflare should help with this
        if (bereq.url ~ "\.(jpg|jpeg|gif|png|ico|css|zip|tgz|gz|rar|bz2|pdf|tar|wav|bmp|rtf|js|flv|swf|html|htm)$") {
                set beresp.ttl = 1h;
        }

        // the content is all public so we never care about cookies
        unset beresp.http.set-cookie;

        // Set the TTL for cache object to five minutes
        set beresp.ttl = 5m;
}

sub vcl_hash {
        hash_data(req.http.X-Forwarded-Proto);
}

sub vcl_deliver {
        if (obj.hits > 0) {
                set resp.http.X-Cache = resp.http.Age;
        } else {
                set resp.http.X-Cache = "MISS";
        }

        unset resp.http.Age;
        unset resp.http.X-Varnish;
        unset resp.http.WP-Super-Cache;
        unset resp.http.Via;
}
