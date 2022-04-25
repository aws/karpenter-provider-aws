// Adapted from code by Matt Walters https://www.mattwalters.net/posts/hugo-and-lunr/
import lunr from 'lunr';

export class OfflineSearch {
  constructor() {
    this.lunrIndex = null;
    this.resultDetails = new Map();
    this.viewingVersion = window.location.pathname.split('/')[1]
    this.$searchInput = $('.td-search-input');

    this.loadIndex(this.$searchInput.data('offline-search-index-json-src'), this.resultDetails);
    this.configureSearchInput();

    this.$searchInput.on('change', (event) => {
      this.render($(event.target));
      // Hide keyboard on mobile browser
      this.$searchInput.blur();
    });
  }

  configureSearchInput() {
    this.$searchInput.data('html', true);
    this.$searchInput.data('placement', 'bottom');
    this.$searchInput.data(
      'template',
      `<div class="popover offline-search-result" role="tooltip">
        <div class="arrow"></div>
        <h3 class="popover-header"></h3>
        <div class="popover-body"></div>
      </div>`
    );

    // Prevent reloading page by enter key on sidebar search.
    this.$searchInput.closest('form').on('submit', () => {
      return false;
    });
  }

  loadIndex(indexSrc, resultDetails) {
    $.ajax(indexSrc).then(
      (data) => {
        this.lunrIndex = lunr(function () {
          this.ref('ref');

          // If you added more searchable fields to the search index, list them here.
          // Here you can specify searchable fields to the search index - e.g. individual toxonomies for you project
          // With "boost" you can add weighting for specific (default weighting without boost: 1)
          this.field('title', {boost: 5});
          this.field('categories', {boost: 3});
          this.field('tags', {boost: 3});
          this.field('description', {boost: 2});
          this.field('body');

          data.forEach((doc) => {
            this.add(doc);
            resultDetails.set(doc.ref, {
              title: doc.title,
              excerpt: doc.excerpt,
            });
          });
        });

        this.$searchInput.trigger('change');
      }
    );
  }

  search(searchQuery, maxResults) {
    return this.lunrIndex.query((lunrQuery) => {
      const tokens = lunr.tokenizer(searchQuery.toLowerCase());
      tokens.forEach((token) => {
        const tokenString = token.toString();
        lunrQuery.term(tokenString, {
          boost: 100,
        });
        lunrQuery.term(tokenString, {
          wildcard:
            lunr.Query.wildcard.LEADING |
            lunr.Query.wildcard.TRAILING,
          boost: 10,
        });
        lunrQuery.term(this.viewingVersion, {
          fields: ["tags"],
          boost: 25
        });
        lunrQuery.term(tokenString, {
          editDistance: 2,
        });
      });
    })
      .slice(0, maxResults);
  }

  render(targetSearchInput) {
    // Dispose the previous result
    targetSearchInput.popover('dispose');

    // Search
    if (this.lunrIndex === null) {
      return
    }

    const searchQuery = targetSearchInput.val();
    const maxResults = targetSearchInput.data('offline-search-max-results');
    if (searchQuery === '') {
      return
    }

    // Search local index
    const results = this.search(searchQuery, maxResults)

    // Make result html
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
            .css({fontWeight: 'bold'})
        )
        .append(
          $('<i>')
            .addClass('fas fa-times search-result-close-button')
            .css({
              cursor: 'pointer',
            })
        )
    );

    const $searchResultBody = $('<div>').css({
      maxHeight: `calc(100vh - ${
        targetSearchInput.offset().top -
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
      results.forEach((result) => {
        $searchResultBody.append(this.renderResult(result));
      });
    }

    targetSearchInput.on('shown.bs.popover', () => {
      $('.search-result-close-button').on('click', () => {
        targetSearchInput.val('');
        targetSearchInput.trigger('change');
      });
    });

    // Enable inline styles in popover.
    const whiteList = $.fn.tooltip.Constructor.Default.whiteList;
    whiteList['*'].push('style');

    targetSearchInput
      .data('content', $html[0].outerHTML)
      .popover({whiteList: whiteList})
      .popover('show');
  }

  renderResult(result) {
    const doc = this.resultDetails.get(result.ref);
    const href =
      this.$searchInput.data('offline-search-base-href') +
      result.ref.replace(/^\//, '');
    const $entry = $('<div>').addClass('mt-4');

    $entry.append(
      $('<small>').addClass('d-block text-muted').text(result.ref)
    );
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

    return $entry;
  }
}