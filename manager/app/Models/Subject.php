<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

class Subject extends Model
{
    use HasFactory;

    protected $fillable = [
        'subject',
        'keywords',
        'is_active',
        'podcasts_count',
        'last_used_at'
    ];
}
