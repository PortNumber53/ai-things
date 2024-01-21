<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

class Task extends Model
{
    use HasFactory;

    protected $casts = [
        'prompt' => 'array',
        'result' => 'array',
        'meta' => 'array',
    ];

    protected $fillable = [
        'task_title',
        'uuid',
        'prompt',
        'result',
        'status',
        'owner_id',
        'meta',
    ];
}
