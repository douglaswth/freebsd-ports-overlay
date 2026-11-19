PORTNAME=	gitea-runner
DISTVERSIONPREFIX=	v
DISTVERSION=	6.0.0
CATEGORIES=	devel
MASTER_SITES=	https://gitea.com/gitea/runner/archive/:src
DISTFILES=	${DISTVERSIONFULL}.tar.gz:src

MAINTAINER=	douglas@douglasthrift.net
COMMENT=	Runner for Gitea Actions
WWW=		https://gitea.com/gitea/runner

LICENSE=	APACHE20 BSD3CLAUSE MIT
LICENSE_COMB=	multi
LICENSE_FILE_APACHE20=	${WRKSRC}/act/container/DOCKER_LICENSE
LICENSE_FILE_BSD3CLAUSE=	${WRKSRC}/act/lookpath/LICENSE
LICENSE_FILE_MIT=	${WRKSRC}/LICENSE

USES=		go:1.27+,modules
USE_RC_SUBR=	gitea_runner

GO_MOD_DIST=	https://gitea.com/gitea/runner/raw/tag/${DISTVERSIONFULL}/
GO_MODULE=	gitea.com/gitea/runner
GO_LDFLAGS=	-X ${GO_MODULE}/internal/pkg/ver.version=${DISTVERSIONFULL}

SUB_FILES=	pkg-message
SUB_LIST=	GITEA_RUNNER_GROUP=${GITEA_RUNNER_GROUP} \
		GITEA_RUNNER_USER=${GITEA_RUNNER_USER}

WRKSRC=		${WRKDIR}/runner

USERS=		${GITEA_RUNNER_USER}
GROUPS=		${GITEA_RUNNER_GROUP}

PLIST_SUB=	GITEA_RUNNER_GROUP=${GITEA_RUNNER_GROUP} \
		GITEA_RUNNER_USER=${GITEA_RUNNER_USER}

OPTIONS_DEFINE=	EXAMPLES

GITEA_RUNNER_USER?=	gitea-runner
GITEA_RUNNER_GROUP?=	gitea-runner

post-install:
	${MKDIR} ${STAGEDIR}${ETCDIR} \
		${STAGEDIR}${PREFIX}/etc/newsyslog.conf.d \
		${STAGEDIR}${PREFIX}/etc/syslog.d \
		${STAGEDIR}/var/db/gitea-runner
	${INSTALL_DATA} ${FILESDIR}/newsyslog.conf \
		${STAGEDIR}${PREFIX}/etc/newsyslog.conf.d/gitea-runner.conf.sample
	${INSTALL_DATA} ${FILESDIR}/syslog.conf \
		${STAGEDIR}${PREFIX}/etc/syslog.d/gitea-runner.conf.sample

post-install-EXAMPLES-on:
	${MKDIR} ${STAGEDIR}${EXAMPLESDIR}
	${INSTALL_DATA} ${WRKSRC}/internal/pkg/config/config.example.yaml \
		${STAGEDIR}${EXAMPLESDIR}

.include <bsd.port.mk>
