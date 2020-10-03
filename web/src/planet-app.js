import { PolymerElement, html } from '@polymer/polymer/polymer-element.js';
import { setPassiveTouchGestures, setRootPath } from '@polymer/polymer/lib/utils/settings.js';
import '@polymer/app-layout/app-drawer/app-drawer.js';
import '@polymer/app-layout/app-drawer-layout/app-drawer-layout.js';
import '@polymer/app-layout/app-header/app-header.js';
import '@polymer/app-layout/app-scroll-effects/app-scroll-effects.js';
import '@polymer/app-layout/app-toolbar/app-toolbar.js';
import '@polymer/paper-icon-button/paper-icon-button.js';
import '@polymer/paper-button/paper-button.js';
import '@polymer/paper-input/paper-input.js';
import '@polymer/paper-slider/paper-slider.js';
import '@polymer/paper-spinner/paper-spinner.js';
import '@polymer/iron-ajax/iron-ajax.js';
import '@polymer/iron-icon/iron-icon.js';
import '@polymer/iron-icons/iron-icons.js';
import './pl-icons.js';
import moment from 'moment/src/moment.js';

import 'leaflet/src/control';
import 'leaflet/src/core';
import 'leaflet/src/layer';
import { Map } from 'leaflet/src/map';
import { TileLayer } from 'leaflet/src/layer/tile';

// Gesture events like tap and track generated from touch will not be
// preventable, allowing for better scrolling performance.
setPassiveTouchGestures(true);

// Set Polymer's root path to the same value we passed to our service worker
// in `index.html`.
setRootPath(PlanetAppGlobals.rootPath);

class PlanetApp extends PolymerElement {
  static get template() {
    return html`

      <!-- FIXME: Include from locally served copy -->
      <link rel="stylesheet" href="https://unpkg.com/leaflet@1.3.1/dist/leaflet.css" />
      <link rel="stylesheet" href="https://unpkg.com/leaflet.markercluster@1.3.0/dist/MarkerCluster.css" media="screen">
      <link rel="stylesheet" href="https://unpkg.com/leaflet.markercluster@1.3.0/dist/MarkerCluster.Default.css" media="screen">

      <style>
        :host {
          --app-primary-color: #009da5;
          --app-secondary-color: black;
          --app-drawer-width: 450px;
          display: block;
        }

        [hidden] {
          display: none !important;
        }

        app-header {
          color: #fff;
          background-color: var(--app-primary-color);
        }

        .white-overlay {
          background-color: white;
          border-radius: 5px;
        }

        #map-overlay-top {
          position: absolute;
          z-index: 999;
          left: 7px;
          top: 80px;
        }

        #map-overlay-bottom {
          position: absolute;
          z-index: 999;
          right: 7px;
          bottom: 20px;
        }

        .map-loading {
          padding: 5px;
        }

        #map {
          position: absolute;
          left: 0;
          right: 0;
          top: 0;
          bottom: 0;
        }

        app-header {
          height: 64px;
        }

        #sidebar {
          display: flex;
          flex-direction: column;
          height: calc(100% - 64px);
        }

        #loading {
          position: absolute;
          top: 0;
          bottom: 0;
          left: 0;
          right: 0;
          z-index: 99;
          display: flex;
          align-items: center;
          justify-content: center;
          background-color: rgba(255, 255, 255, 0.7);
        }

        #loading > paper-spinner {
          height: 50px;
          width: 50px;
        }

        #sidebar-top {
          flex: 0 1 auto;
        }

        #results-container {
          flex: 1 1 auto;
          position: relative;
        }
        #results {
          position: absolute;
          top: 0;
          bottom: 0;
          left: 0;
          right: 0;
          overflow: auto;
        }

        .result {
          margin: 10px;
          padding: 5px;
          background-color: #eee;
          display: flex;
        }

        .thumb {
          padding-right: 10px;
        }
        .thumb > img {
          width: 150px;
          height: 150px;
        }

        .currently-showing {
          padding: 5px;
          transition-delay: 1s;
        }


        .error-container {
          padding: 10px;
        }
        .error {
          color: maroon;
          font-weight: bold;
        }

        .error-message {
          font-family: monospace;
          padding: 5px;
          font-size: small;
          color: 666;
        }

        .icon-container {
          display: flex;
          align-items: center;
        }
        .icon-container > iron-icon {
          padding-right: 5px;
        }
      </style>

      <iron-ajax id="search" handle-as="json" on-response="handleSearch_" on-error="handleSearchError_" url="/api/search" params="[[params_]]" auto="[[params_]]" debounce-duration="300" loading="{{loading_}}"></iron-ajax>
      <iron-ajax id="apiKeyUpdate" url="/api/key" method="POST" handle-as="text"></iron-ajax>

      <app-drawer-layout fullbleed="" force-narrow="[[forceNarrow_]]">
        <app-drawer id="drawer" slot="drawer" swipe-open="">
          <app-header fixed="">
            <app-toolbar>Planet Data Viewer</app-toolbar>
          </app-header>
          <div id="sidebar">
            <div id="sidebar-top">

              <div class="currently-showing" hidden="[[!tileUrl_]]">
                <div>
                  Showing <b>[[tileName_]]</b>
                </div>
                <div class="buttons">
                  <paper-button raised="" on-tap="clearTiles_">Clear</paper-button>
                  <paper-button raised="" on-tap="openCaltopo_">Open View in CalTopo</paper-button>
                </div>
                <div>
                  <small>
                    The CalTopo generated link is brittle so I wouldn't be surprised if it breaks in the future.
                  </small>
                </div>
                <div>
                  Satellite Opacity ([[opacity]]%)
                  <paper-slider min="0" max="100" value="[[opacity]]" immediate-value="{{opacity}}"></paper-slider>
                </div>
              </div>
            </div>


            <div id="results-container">
              <div id="loading" hidden$="[[!loading_]]">
                <paper-spinner active=""></paper-spinner>
              </div>
              <div id="results">
                <template is="dom-repeat" items="[[search]]">
                  <div class="result">
                    <div class="thumb">
                      <img src="[[item.thumb]]">
                    </div>
                    <div>
                      <div><b>[[toDate(item.acquired)]]</b></div>
                      <div><b>[[toTime(item.acquired)]]</b></div>
                      <div><i>Captured [[toDelta(item.acquired)]] ago</i></div>
                      <div>Visibility: <b>[[item.clear_percent]]%</b></div>
                      <div>
                        <paper-button data-name$="[[toName(item)]]" data-url$="[[item.tile_url]]" raised="" on-tap="loadTiles_">Load</paper-button>
                      </div>
                    </div>
                  </div>
                </template>

                <div hidden$="[[!isError]]" class="error-container">
                  <div class="error icon-container">
                    <span>
                      <iron-icon icon="error"></iron-icon>
                      Problem Loading Satellite Imagery
                    </span>
                  </div>
                  <div class="error-message">[[errorMessage]]</div>
                  <div hidden$="[[!isApiKeyError]]">
                    <p>
                    <div>It looks like we need a new Planet API key! You can help!</div>
                    <ol>
                      <li>
                        Go to <a href="https://www.planet.com/login/" target="_blank">https://www.planet.com</a>.
                        Click <em>Sign Up</em> and create a trial account.
                        <p>
                      </li>
                      <li>
                        Open <em>My Account</em>
                        <div>
                          <img src="/images/planet_myaccount.png">
                        </div>
                        <p>
                      </li>
                      <li>
                        Copy the <em>API Key</em>
                        <div>
                          <img width="350" src="/images/planet_apikey.png">
                        </div>
                        <p>
                      </li>
                      <li>
                        Paste it here:
                        <div>
                          <paper-input id="apiKey" label="Enter API Key"></paper-input>
                          <paper-button raised="" on-tap="onApiKey_">SAVE</paper-button>
                          <paper-spinner id="apiKeySpinner"></paper-spinner>
                        </div>
                        <p>
                      </li>
                    </ol>
                



                  </div>
                </div>
              </div>
            </div>
 

          </div>
        </app-drawer>
        <div id="map-overlay-top">
          <paper-icon-button class="white-overlay" icon="pl-icons:menu" on-tap="drawerToggle_"></paper-icon-button>
        </div>
        <div id="map-overlay-bottom">
          <div class="map-loading white-overlay" hidden$="[[!mapLoading_]]">
            Loading Map Tiles... 
            <div>
            <paper-spinner active=""></paper-spinner>
            </div>
          </div>
        </div>
        <div id="map">
        </div>
      </app-drawer-layout>
    `;
  }

  static get properties() {
    return {
      forceNarrow_: Boolean,

      opacity: {
        type: Number,
        value: 100,
        observer: 'onOpacity_',
      },

      map: {
        type: Object,
      },

      tileName_: {
        type: String,
      },
      tileUrl_: {
        type: String,
        value: '',
      },

      planetLayer: {
        type: Object,
      },

      zoom: {
        type: Number,
      },
      bounds: {
        type: Object,
      },

      params_: {
        type: Object,
      },

      search: {
        type: Object,
      },

      loading_: {
        type: Boolean,
        value: false,
      },
      mapLoading_: {
        type: Boolean,
        value: false,
      },

      isError: {
        type: Boolean,
        value: false,
      },
      errorMessage: {
        type: String,
        value: "",
      },
      isApiKeyError: {
        type: Boolean,
        value: false,
      },
    };
  }

  toName(r) {
    return this.toDate(r.acquired);
  }

  toDate(ts) {
    const m = moment(ts);
    return m.format("dddd") + ", " + m.format("LL");
  }
  toTime(ts) {
    return moment(ts).format("LT");
  }
  toDelta(ts) {
    const now = moment();
    const m = moment(ts);
    return moment.duration(now.diff(m)).humanize();
  }

  handleSearch_(e) {
    this.$.results.scrollTop = 0;  // scroll back to top

    if (e.detail.response && e.detail.response.results) {
      this.isError = false;
      this.search = e.detail.response.results;
    }
  }

  handleSearchError_(e) {
    this.$.results.scrollTop = 0;  // scroll back to top

    this.isError = true;
    this.search = [];

    const resp = e.detail.request.xhr.response;

    this.errorMessage = resp.error;
    const keyRe = /\bkey\b/g;
    this.isApiKeyError = !!resp.error.match(keyRe);
  }

  loadTiles_(e) {
    this.tileName_ = e.target.dataset.name;
    this.tileUrl_ = e.target.dataset.url;
    this.planetLayer.setUrl(this.tileUrl_);
  }

  clearTiles_(e) {
    this.tileName_ = '';
    this.tileUrl_ = '';
    this.planetLayer.setUrl('');
  }

  openCaltopo_() {
    const center = this.bounds.getCenter();

    const name = "Planet - " + this.tileName_;
    var tiles = 'https://planet.jeffheidel.com' + this.tileUrl_;
    tiles = tiles.replace("{x}", `{X}`);
    tiles = tiles.replace("{y}", `{Y}`);
    tiles = tiles.replace("{z}", `{Z}`);

    const t1 = '{"template":"' + tiles + '","type":"TILE","maxzoom":"20"}'
    const t2 = '{"custom":[{"properties":{"title":"' + name + '","template":"' + tiles + '","type":"TILE","maxzoom":"20","alphaOverlay":false,"class":"CustomLayer"},"id":""}]}';
    const enc = encodeURIComponent(encodeURIComponent(t1)) + '&n=1&cl=' + encodeURIComponent(t2);
    const url = 'https://caltopo.com/map.html#ll=' + center.lat + ',' + center.lng + '&z=' + this.zoom + '&b=mbt&o=cl_' + enc;

    window.open(url, '_blank');
  }

  newBounds_(zoom, bounds) {
    this.zoom = zoom;
    this.bounds = bounds;

    const center = this.bounds.getCenter();

    this.params_ = {
      'lat': center.lat,
      'lng': center.lng,
      'z': this.zoom,
      'group_by': 'date',
    };
  }

  connectedCallback() {
    super.connectedCallback();

    this.map = new Map(this.$.map, {
      center: [47.5, -119],
      zoom: 7,
      inertiaDeceleration: 3000,
      inertiaMaxSpeed: 3000,
      tapTolerance: 40,
      tap: false
    });
    this.map.on('moveend', () => {
      this.newBounds_(this.map.getZoom(), this.map.getBounds());
    });
    this.map.setView([47.5, -119], 7);


    const baseLayer = new TileLayer('https://tile.thunderforest.com/landscape/{z}/{x}/{y}.png?apikey=b99b298d147e4c8fafd7929f48e816cc', {
        attribution: '&copy; <a href="https://www.thunderforest.com/">Thunderforest</a>'
    });
    baseLayer.addTo(this.map);

    this.planetLayer = new TileLayer('', {
        attribution: '&copy; <a href="https://www.planet.com/">Planet</a>'
    });
    this.planetLayer.addTo(this.map);

    this.planetLayer.on('loading', () => {
      if (this.tileUrl_) {
        this.mapLoading_ = true;
      }
    });
    this.planetLayer.on('load', () => {
      this.mapLoading_ = false;
    });

    // TODO need viewport to issue ajax request....
  }

  onOpacity_(v) {
    if (!this.planetLayer) {
      return;
    }
    this.planetLayer.setOpacity(v / 100);
  }

  drawerToggle_() {
    console.log("toggle");
    this.$.drawer.toggle();
    this.forceNarrow_ = !this.$.drawer.opened;
  }

  onApiKey_() {
    const value = this.$.apiKey.value.trim();
    const ajax = this.$.apiKeyUpdate;
    ajax.body = 'key=' + value;
    ajax.generateRequest();
    this.$.apiKeySpinner.active = true;

    // A bit ugly, but try the API request again after a reasonable amount of time.
    setTimeout(() => {
      this.$.search.generateRequest();
      this.$.apiKey.value = "";
      this.$.apiKeySpinner.active = false;
    }, 1000);
  }
}

window.customElements.define('planet-app', PlanetApp);
