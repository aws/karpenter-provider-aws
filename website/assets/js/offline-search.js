// Adapted from code by Matt Walters https://www.mattwalters.net/posts/2018-03-28-hugo-and-lunr/
// Modified to filter search results to only show those from the currently selected version.

(function ($) {
    'use strict';

    // Detect the current version from the URL path.
    // The first path segment is typically the version (e.g., "v1.9", "preview", "docs").
    // On the homepage (no path segments), default to "docs" (the latest version's content).
    function getCurrentVersion() {
        const pathSegments = window.location.pathname.split('/').filter(Boolean);
        if (pathSegments.length > 0) {
            return pathSegments[0];
        }
        // On the homepage, default to showing results from the latest version ("docs")
        return 'docs';
    }

    // Extract the version prefix from a search result ref (e.g., "/v1.9/concepts/..." -> "v1.9").
    function getVersionFromRef(ref) {
        const segments = ref.split('/').filter(Boolean);
        if (segments.length > 0) {
            return segments[0];
        }
        return '';
    }

    // Filter results to only include those from the current version.
    function filterToCurrentVersion(results, currentVersion) {
        if (!currentVersion) {
            return results;
        }

        return results.filter((r) => {
            const resultVersion = getVersionFromRef(r.ref);
            return resultVersion === currentVersion;
        });
    }

    $(document).ready(function () {
        const $searchInput = $('.td-search input');

        //
        // Options for popover
        //

        $searchInput.data('html', true);
        $searchInput.data('placement', 'bottom');
        $searchInput.data(
            'template',
            '<div class="td-offline-search-results popover" role="tooltip"><div class="arrow"></div><h3 class="popover-header"></h3><div class="popover-body"></div></div>'
        );

        //
        // Register handler
        //

        $searchInput.on('change', (event) => {
            render($(event.target));

            // Hide keyboard on mobile browser
            $searchInput.blur();
        });

        // Prevent reloading page by enter key on sidebar search.
        $searchInput.closest('form').on('submit', () => {
            return false;
        });

        //
        // Lunr
        //

        let idx = null; // Lunr index
        const resultDetails = new Map(); // Will hold the data for the search results (titles and summaries)

        // Set up for an Ajax call to request the JSON data file that is created by Hugo's build process
        $.ajax($searchInput.data('offline-search-index-json-src')).then(
            (data) => {
                idx = lunr(function () {
                    this.ref('ref');

                    // If you added more searchable fields to the search index, list them here.
                    // Here you can specify searchable fields to the search index - e.g. individual toxonomies for you project
                    // With "boost" you can add weighting for specific (default weighting without boost: 1)
                    this.field('title', { boost: 5 });
                    this.field('categories', { boost: 3 });
                    this.field('tags', { boost: 3 });
                    // this.field('projects', { boost: 3 }); // example for an individual toxonomy called projects
                    this.field('description', { boost: 2 });
                    this.field('body');

                    data.forEach((doc) => {
                        this.add(doc);

                        resultDetails.set(doc.ref, {
                            title: doc.title,
                            excerpt: doc.excerpt,
                        });
                    });
                });

                $searchInput.trigger('change');
            }
        );

        const render = ($targetSearchInput) => {
            // Dispose the previous result
            $targetSearchInput.popover('dispose');

            //
            // Search
            //

            if (idx === null) {
                return;
            }

            const searchQuery = $targetSearchInput.val();
            if (searchQuery === '') {
                return;
            }

            const rawResults = idx
                .query((q) => {
                    const tokens = lunr.tokenizer(searchQuery.toLowerCase());
                    tokens.forEach((token) => {
                        const queryString = token.toString();
                        q.term(queryString, {
                            boost: 100,
                        });
                        q.term(queryString, {
                            wildcard:
                                lunr.Query.wildcard.LEADING |
                                lunr.Query.wildcard.TRAILING,
                            boost: 10,
                        });
                        q.term(queryString, {
                            editDistance: 2,
                        });
                    });
                });

            // Filter to current version FIRST, then slice to max results.
            // This ensures we get the full set of relevant results for the
            // selected version instead of slicing across all versions first.
            const currentVersion = getCurrentVersion();
            const results = filterToCurrentVersion(rawResults, currentVersion)
                .slice(
                    0,
                    $targetSearchInput.data('offline-search-max-results')
                );

            //
            // Make result html
            //

            const $html = $('<div>');

            $html.append(
                $('<div>')
                    .css({
                        display: 'flex',
                        justifyContent: 'space-between',
                        marginBottom: '1em',
                    })
                    .append(
                        $('<span>')
                            .text('Search results')
                            .css({ fontWeight: 'bold' })
                    )
                    .append(
                        $('<span>')
                            .addClass('td-offline-search-results__close-button')
                    )
            );

            const $searchResultBody = $('<div>').css({
                maxHeight: `calc(100vh - ${
                    $targetSearchInput.offset().top -
                    $(window).scrollTop() +
                    180
                }px)`,
                overflowY: 'auto',
            });
            $html.append($searchResultBody);

            if (results.length === 0) {
                $searchResultBody.append(
                    $('<p>').text(`No results found for query "${searchQuery}"`)
                );
            } else {
                results.forEach((r) => {
                    const doc = resultDetails.get(r.ref);
                    const href =
                        $searchInput.data('offline-search-base-href') +
                        r.ref.replace(/^\//, '');

                    const resultVersion = getVersionFromRef(r.ref);
                    const $entry = $('<div>').addClass('mt-4');

                    // Show version badge and path
                    const $meta = $('<small>').addClass('d-block text-muted');
                    if (resultVersion) {
                        const $versionBadge = $('<span>')
                            .text(resultVersion)
                            .css({
                                display: 'inline-block',
                                padding: '0 0.4em',
                                marginRight: '0.5em',
                                fontSize: '0.85em',
                                fontWeight: '600',
                                borderRadius: '3px',
                                backgroundColor: resultVersion === currentVersion ? '#0d6efd' : '#6c757d',
                                color: '#fff',
                            });
                        $meta.append($versionBadge);
                    }
                    $meta.append(document.createTextNode(r.ref));
                    $entry.append($meta);

                    $entry.append(
                        $('<a>')
                            .addClass('d-block')
                            .css({
                                fontSize: '1.2rem',
                            })
                            .attr('href', href)
                            .text(doc.title)
                    );

                    $entry.append($('<p>').text(doc.excerpt));

                    $searchResultBody.append($entry);
                });
            }

            $targetSearchInput.on('shown.bs.popover', () => {
                $('.td-offline-search-results__close-button').on('click', () => {
                    $targetSearchInput.val('');
                    $targetSearchInput.trigger('change');
                });
            });

            $targetSearchInput
                .data('content', $html[0])
                .popover('show');
        };
    });
})(jQuery);
