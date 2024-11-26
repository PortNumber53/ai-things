<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

class Subscriptions extends Model
{
    use HasFactory;

    protected $fillable = [
        'feed_url',
        'title',
        'description',
        'site_url',
        'last_fetched_at',
        'last_build_date',
        'is_active'
    ];

    protected $casts = [
        'is_active' => 'boolean',
        'last_fetched_at' => 'datetime',
        'last_build_date' => 'datetime'
    ];
}
