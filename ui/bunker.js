

angular.module('bunker', [])
    .controller('bunker', ['$scope', '$http', function ($scope, $http) {
        $scope.baseurl = "http://localhost:9000/api/v1/kv/"
        $scope.url = []
        $scope.keys = []
        $scope.nodes = []
        $scope.active = "/"
        $scope.activeItem = null
        $scope.getkeys = function (path) {
            if (path == '') {
                $scope.url = []
            } else {
                $scope.url = path.split("/")
            }
            $http({
                method: 'GET',
                url: $scope.baseurl + path
            }).then(
                function (res) {
                    console.log(res)
                    items = res.data
                    keys = []
                    for (k in res.data) {
                        if ($scope.isDir(res.data[k]) == "dir") {
                            keys.push({
                                "key": res.data[k],
                                "dir": true,
                            })
                        } else {
                            keys.push({
                                "key": res.data[k],
                                "dir": false,
                            })
                        }
                    }
                    $scope.keys = keys
                    return keys
                }, function (res) {
                    console.log(res)
                }
            )
        }

        $scope.getnodes = function () {
            $http({
                method: 'GET',
                url: "http://localhost:9000/api/v1/cluster/nodes"
            }).then(function (res) {
                $scope.nodes = res.data
            }, function (res) {
                console.log(res)
            })

        }

        $scope.clickKey = function (key) {
            $scope.active = key
            uri = $scope.url.join("/") + key.key
            if (key.dir) {

                $scope.getkeys(uri)
                return
            } else {
                $http({
                    method: 'GET',
                    url: $scope.baseurl + uri,
                }).then(function (res) {
                    if (res.data.secret != undefined && $scope.decrypt) {
                        $http({
                            method: 'GET',
                            url: $scope.baseurl + uri,
                            params: {
                                secret: true
                            }
                        }).then(function (res) {
                            $scope.activeItem = JSON.stringify(res.data, undefined, 4)
                        }, function (res) {
                            console.log(res)
                        })
                    }
                    $scope.activeItem = JSON.stringify(res.data, undefined, 4)
                }, function (res) {
                    console.log(res)
                })
            }
        }

        $scope.isDir = function (item) {
            if (item.endsWith("/")) {
                return "dir"
            }
            return ""
        }

        $scope.stripSlash = function (item) {
            return item.replace("/", "")
        }

        $scope.breadcrumbClick = function (crumb) {
            idx = $scope.url.indexOf(crumb)
            url = []
            for (var i = 0; i <= idx; i++) {
                url.push($scope.url[i])
            }
            $scope.getkeys(url.join("/") + "/")
        }
        $scope.getnodes()
        $scope.getkeys("")
    }])