<?php
// PHP web challenge service for the production fleet smoke.
// Implements the same HTTP contract as examples/sample-http-challenge so the
// bundled sample-http checker can drive put/get/check against it. The /leak
// route is the deliberately insecure attack path used by the smoke.
//
// Served with the PHP built-in web server:
//   php -S 0.0.0.0:$PORT router.php

declare(strict_types=1);

const MAX_BODY_BYTES = 1048576;

function state_dir(): string
{
    $dir = getenv('AD_STATE_DIR');
    return $dir !== false && $dir !== '' ? $dir : '/opt/ad/state';
}

function state_path(): string
{
    return rtrim(state_dir(), '/') . '/fleet-web-php-state.json';
}

function checker_token(): string
{
    $token = getenv('AD_CHECKER_TOKEN');
    return $token !== false && $token !== '' ? $token : 'sample-http-checker-token';
}

function load_flag(): string
{
    $path = state_path();
    if (!is_file($path)) {
        return '';
    }
    $raw = @file_get_contents($path);
    if ($raw === false) {
        return '';
    }
    $data = json_decode($raw, true);
    if (is_array($data) && isset($data['flag']) && is_string($data['flag'])) {
        return $data['flag'];
    }
    return '';
}

function save_flag(string $flag): void
{
    $dir = state_dir();
    if (!is_dir($dir)) {
        @mkdir($dir, 0o755, true);
    }
    file_put_contents(state_path(), json_encode(['flag' => $flag]));
}

function send_json(int $status, array $payload): void
{
    http_response_code($status);
    header('Content-Type: application/json');
    echo json_encode($payload);
}

function header_token(): string
{
    if (isset($_SERVER['HTTP_X_CHECKER_TOKEN'])) {
        return (string) $_SERVER['HTTP_X_CHECKER_TOKEN'];
    }
    return '';
}

$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$route = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);

if ($method === 'GET' && $route === '/health') {
    send_json(200, ['status' => 'ok']);
    return true;
}

if ($method === 'GET' && $route === '/flag') {
    if (!hash_equals(checker_token(), header_token())) {
        send_json(403, ['status' => 'forbidden']);
        return true;
    }
    $flag = load_flag();
    if ($flag !== '') {
        send_json(200, ['status' => 'stored', 'flag' => $flag]);
    } else {
        send_json(404, ['status' => 'missing']);
    }
    return true;
}

if ($method === 'GET' && $route === '/leak') {
    $flag = load_flag();
    if ($flag !== '') {
        send_json(200, ['status' => 'leaked', 'flag' => $flag]);
    } else {
        send_json(404, ['status' => 'missing']);
    }
    return true;
}

if ($method === 'POST' && $route === '/flag') {
    if (!hash_equals(checker_token(), header_token())) {
        send_json(403, ['status' => 'forbidden']);
        return true;
    }
    $raw = file_get_contents('php://input', false, null, 0, MAX_BODY_BYTES + 1);
    if ($raw === false) {
        $raw = '';
    }
    if (strlen($raw) > MAX_BODY_BYTES) {
        send_json(413, ['status' => 'too_large']);
        return true;
    }
    $payload = json_decode($raw === '' ? '{}' : $raw, true);
    if (!is_array($payload)) {
        send_json(400, ['status' => 'invalid_json']);
        return true;
    }
    $flag = $payload['flag'] ?? '';
    if (!is_string($flag) || $flag === '') {
        send_json(400, ['status' => 'missing_flag']);
        return true;
    }
    save_flag($flag);
    send_json(200, ['status' => 'stored', 'flag' => $flag]);
    return true;
}

send_json(404, ['status' => 'not_found']);
return true;
