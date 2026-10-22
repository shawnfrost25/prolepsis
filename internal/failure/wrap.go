package failure

import "fmt"

// FORMAT: <application>.<component>.<resource>.<subresource>.<operation>
//                                              ^^^^^^^^^^^^^ (singluar (not plural) / optional)

// To don't mistake:
// -- op -> Explain the general scope (e.g. get_user_by_id, github_redirect)
// -- code -> Explain the cause (e.g. user_query_failed, id_parameter_malformed)
// -- wrap -> Explain the current process (e.g. kitanai.http.parameters.id.convert, github.redis.webhook.secret.fetch)
func Wrap(process string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", process, err)
}
