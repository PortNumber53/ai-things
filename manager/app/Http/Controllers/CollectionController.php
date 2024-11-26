<?php

namespace App\Http\Controllers;

use App\Http\Requests\StorecollectionRequest;
use App\Http\Requests\UpdatecollectionRequest;
use App\Models\collection;

class CollectionController extends Controller
{
    /**
     * Display a listing of the resource.
     */
    public function index()
    {
        //
    }

    /**
     * Show the form for creating a new resource.
     */
    public function create()
    {
        //
    }

    /**
     * Store a newly created resource in storage.
     */
    public function store(StorecollectionRequest $request)
    {
        //
    }

    /**
     * Display the specified resource.
     */
    public function show(collection $collection)
    {
        //
    }

    /**
     * Show the form for editing the specified resource.
     */
    public function edit(collection $collection)
    {
        //
    }

    /**
     * Update the specified resource in storage.
     */
    public function update(UpdatecollectionRequest $request, collection $collection)
    {
        //
    }

    /**
     * Remove the specified resource from storage.
     */
    public function destroy(collection $collection)
    {
        //
    }
}
