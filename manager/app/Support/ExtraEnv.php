<?php

namespace App\Support;

use Dotenv\Dotenv;

final class ExtraEnv
{
    private const FILENAME = '_extra_env';

    public static function load(): void
    {
        $basePath = base_path();
        $candidates = [
            $basePath,
            dirname($basePath),
        ];

        foreach ($candidates as $candidate) {
            $path = $candidate . DIRECTORY_SEPARATOR . self::FILENAME;
            if (!is_file($path)) {
                continue;
            }

            Dotenv::createImmutable($candidate, self::FILENAME)->safeLoad();
            return;
        }
    }
}
